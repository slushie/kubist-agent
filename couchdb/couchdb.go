package couchdb

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

type Client struct {
	*Auth
	c   *http.Client
	url *url.URL
}

type Auth struct {
	Username, Password string
}

type ClientInterface interface {
	Database(name string) DatabaseInterface
}

var _ ClientInterface = &Client{}

type Database struct {
	*Client
	name string
}

type DatabaseInterface interface {
	Exists() (bool, error)
	Create() error
	Drop() error
	Changes(changesCh chan<- Body, stopCh <-chan struct{}) error

	Head(id string) (*StatusObject, error)
	Get(id string) (*StatusObject, error)
	GetOrNil(id string) (*StatusObject, error)
	Delete(doc Body) (*StatusObject, error)
	Post(doc Body) (*StatusObject, error)
	Put(id string, doc Body) (*StatusObject, error)
}

var _ DatabaseInterface = &Database{}

type StatusObject struct {
	*http.Response
	Body
}

var _ error = &StatusObject{}

type Body map[string]interface{}

func NewClient(baseUrl string, auth *Auth) (*Client, error) {
	base, err := url.Parse(baseUrl)
	if err != nil {
		return nil, err
	}

	if base.Path == "" {
		base.Path = "/"
	}

	return &Client{Auth: auth, c: &http.Client{}, url: base}, nil
}

func (c *Client) Info() (*StatusObject, error) {
	if res, err := c.request(http.MethodGet, "", nil); err != nil {
		return nil, err
	} else {
		return c.createStatusObject(res)
	}
}

func (c *Client) Database(name string) DatabaseInterface {
	return &Database{Client: c, name: url.QueryEscape(name)}
}

func (db *Database) Changes(changesCh chan<- Body, stopCh <-chan struct{}) error {
	defer close(changesCh)

	res, err := db.request(http.MethodGet, db.urlFor("_changes"), nil)
	if err != nil {
		return err
	}

	// create a new channel to read lines from body
	scan := bufio.NewScanner(res.Body)
	lineCh := make(chan string)
	go scannerChannel(scan, lineCh)

	for {
		select {
		case <-stopCh:
			return res.Body.Close()

		case line := <-lineCh:
			if line == "" {
				return nil // eof
			}

			var obj Body
			if err := json.Unmarshal(bytes.NewBufferString(line).Bytes(), &obj); err != nil {
				return err
			}

			changesCh <- obj
		}
	}
}

func scannerChannel(s *bufio.Scanner, ch chan<- string) {
	for s.Scan() {
		ch <- s.Text() + "\n"
	}

	close(ch)
}

func (db *Database) Head(id string) (*StatusObject, error) {
	res, err := db.request(http.MethodHead, db.urlFor(id), nil)
	if err != nil {
		return nil, err
	}

	return db.createStatusObject(res)
}

func (db *Database) Get(id string) (*StatusObject, error) {
	res, err := db.request(http.MethodGet, db.urlFor(id), nil)
	if err != nil {
		return nil, err
	}

	return db.parseResponse(res)
}

func (db *Database) GetOrNil(id string) (*StatusObject, error) {
	res, err := db.request(http.MethodGet, db.urlFor(id), nil)
	if err != nil {
		return nil, err
	}

	if res.StatusCode == http.StatusNotFound {
		res.Body.Close()
		return nil, nil
	}

	return db.parseResponse(res)
}

func (db *Database) Delete(doc Body) (*StatusObject, error) {
	id := doc["_id"].(string)
	if id == "" {
		return nil, errors.New("missing doc _id")
	}

	rev := doc["_rev"].(string)
	if rev == "" {
		return nil, errors.New("missing doc _rev")
	}

	req, err := db.createRequest(http.MethodDelete, db.urlFor(id), nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("If-Match", rev)

	res, err := db.c.Do(req)
	if err != nil {
		return nil, err
	}

	return db.parseResponse(res)
}

func (db *Database) Post(doc Body) (*StatusObject, error) {
	res, err := db.request(http.MethodPost, db.name, doc)
	if err != nil {
		return nil, err
	}

	return db.parseResponse(res)
}

func (db *Database) Put(id string, doc Body) (*StatusObject, error) {
	req, err := db.createRequest(http.MethodPut, db.urlFor(id), doc)
	if err != nil {
		return nil, err
	}

	if rev, ok := doc["_rev"].(string); ok {
		req.Header.Set("If-Match", rev)
	}

	res, err := db.c.Do(req)
	if err != nil {
		return nil, err
	}

	return db.parseResponse(res)
}

// Returns true if the database exists.
func (db *Database) Exists() (bool, error) {
	res, err := db.request(http.MethodHead, db.urlFor(""), nil)
	if err != nil {
		return false, err
	}

	status, err := db.createStatusObject(res)
	if err != nil {
		return false, err
	}

	return status.StatusCode == http.StatusOK, nil
}

// Create the database.
func (db *Database) Create() error {
	res, err := db.request(http.MethodPut, db.urlFor(""), nil)
	if err != nil {
		return err
	}

	status, err := db.createStatusObject(res)
	if err != nil {
		return err
	}

	if status.StatusCode == http.StatusCreated {
		return nil // created, no error
	}

	return status
}

// Drop (delete) the database.
func (db *Database) Drop() error {
	res, err := db.request(http.MethodDelete, db.urlFor(""), nil)
	if err != nil {
		return err
	}

	status, err := db.createStatusObject(res)
	if err != nil {
		return err
	}

	if status.StatusCode == http.StatusOK {
		return nil // deleted, no error
	}

	return status
}

func (db *Database) urlFor(id string) string {
	if id == "" {
		return db.name
	}
	return db.name + "/" + url.QueryEscape(id)
}

func (c *Client) request(method, path string, body Body) (*http.Response, error) {
	req, err := c.createRequest(method, path, nil)
	if err != nil {
		return nil, err
	}

	return c.c.Do(req)
}

func (c *Client) createRequest(method, path string, body Body) (*http.Request, error) {
	pathUrl, err := url.Parse(path)
	if err != nil {
		return nil, err
	}
	u := c.url.ResolveReference(pathUrl)

	var content io.Reader = http.NoBody
	if body != nil {
		buf := &bytes.Buffer{}
		enc := json.NewEncoder(buf)
		enc.Encode(body)
		content = buf
	}

	req, err := http.NewRequest(method, u.String(), content)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	c.authorizeRequest(req)

	return req, nil
}

func (c *Client) authorizeRequest(req *http.Request) {
	if c.Auth.Username == "" && c.Auth.Password == "" {
		return
	}

	credentials := c.Auth.Username + ":" + c.Auth.Password
	basicAuth := base64.StdEncoding.EncodeToString([]byte(credentials))
	req.Header.Set("Authorization", "Basic "+basicAuth)
}

func (c *Client) createStatusObject(res *http.Response) (*StatusObject, error) {
	body, err := c.parseJsonBody(res)
	if err != nil {
		return nil, err
	}

	return &StatusObject{res, body}, nil
}

func (c *Client) parseResponse(res *http.Response) (*StatusObject, error) {
	if status, err := c.createStatusObject(res); err != nil {
		res.Body.Close()
		return nil, err
	} else if status.StatusCode >= 400 {
		return nil, status
	} else {
		return status, nil
	}
}

func (*Client) parseJsonBody(res *http.Response) (Body, error) {
	var err error

	buf := &bytes.Buffer{}
	if n, err := buf.ReadFrom(res.Body); err != nil {
		return nil, err
	} else if n == 0 {
		return nil, nil
	}

	var body Body
	if err = json.Unmarshal(buf.Bytes(), &body); err != nil {
		return nil, err
	} else {
		return body, nil
	}
}

func (so *StatusObject) Error() string {
	return fmt.Sprintf("HTTP status %s", so.Status)
}

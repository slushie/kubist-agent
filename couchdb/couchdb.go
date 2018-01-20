package couchdb

import (
	"net/http"
	"bytes"
	"encoding/json"
	"net/url"
	"encoding/base64"
	"errors"
	"bufio"
	"fmt"
)

type Client struct {
	*Auth
	c   *http.Client
	url *url.URL
}

type Database struct {
	*Client
	name string
}

type Auth struct {
	Username, Password string
}

type StatusObject struct {
	*http.Response
	Body BodyObject
}

var _ error = &StatusObject{}

type BodyObject map[string]interface{}

func NewClient(baseUrl string, auth *Auth) (*Client, error) {
	base, err := url.Parse(baseUrl)
	if err != nil {
		return nil, err
	}

	return &Client{Auth: auth, c: &http.Client{}, url: base}, nil
}

func (c *Client) Database(name string) *Database {
	return &Database{Client: c, name: url.QueryEscape(name)}
}

func (db *Database) Changes(changesCh chan<- BodyObject, stopCh <-chan struct{}) error {
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

			var obj BodyObject
			dec := json.NewDecoder(bytes.NewBufferString(line))
			if err := dec.Decode(obj); err != nil {
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

func (db *Database) Get(id string) (BodyObject, error) {
	res, err := db.request(http.MethodGet, db.urlFor(id), nil)
	if err != nil {
		return nil, err
	}

	return db.parseResponse(res)
}

func (db *Database) GetOrNil(id string) (BodyObject, error) {
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

func (db *Database) Delete(doc BodyObject) (BodyObject, error) {
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

func (db *Database) Post(doc BodyObject) (BodyObject, error) {
	res, err := db.request(http.MethodPost, db.name, doc)
	if err != nil {
		return nil, err
	}

	return db.parseResponse(res)
}

func (db *Database) Put(id string, doc BodyObject) (BodyObject, error) {
	res, err := db.request(http.MethodPut, db.urlFor(id), doc)
	if err != nil {
		return nil, err
	}

	return db.parseResponse(res)
}

func (db *Database) urlFor(id string) string {
	return db.name + "/" + url.QueryEscape(id)
}

func (db *Database) request(method, path string, body BodyObject) (*http.Response, error) {
	req, err := db.createRequest(method, path, nil)
	if err != nil {
		return nil, err
	}

	return db.c.Do(req)
}

func (db *Database) createRequest(method, path string, body BodyObject) (*http.Request, error) {
	var serializedBody *bytes.Buffer
	if body != nil {
		serializedBody = &bytes.Buffer{}
		enc := json.NewEncoder(serializedBody)
		enc.Encode(body)
	}

	pathUrl, err := url.Parse(path)
	if err != nil {
		return nil, err
	}
	u := db.url.ResolveReference(pathUrl)

	req, err := http.NewRequest(method, u.String(), serializedBody)
	if err != nil {
		return nil, err
	}

	if serializedBody != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	req.Header.Set("Accept", "application/json")
	db.authorizeRequest(req)

	return req, nil
}

func (db *Database) authorizeRequest(req *http.Request) {
	if db.Auth == nil {
		return
	}

	credentials := db.Auth.Username + ":" + db.Auth.Password
	basicAuth := base64.StdEncoding.EncodeToString([]byte(credentials))
	req.Header.Set("Authorization", "Basic "+basicAuth)
}

func (db *Database) createStatusObject(res *http.Response) (*StatusObject, error) {
	body, err := db.parseJsonBody(res)
	if err != nil { return nil, err }

	return &StatusObject{res, body}, nil
}

func (db *Database) parseResponse(res *http.Response) (BodyObject, error) {
	if status, err := db.createStatusObject(res); err != nil {
		res.Body.Close()
		return nil, err
	} else if status.StatusCode >= 400 {
		return nil, status
	} else {
		return db.parseJsonBody(res)
	}
}

func (db *Database) parseJsonBody(res *http.Response) (BodyObject, error) {
	var err error

	buf := &bytes.Buffer{}
	if _, err = buf.ReadFrom(res.Body); err != nil {
		return nil, err
	}

	var body interface{}
	dec := json.NewDecoder(buf)
	if err = dec.Decode(body); err != nil {
		return nil, err
	}

	switch obj := body.(type) {
	case BodyObject:
		return obj, nil
	default:
		return nil, errors.New("invalid JSON response: " + buf.String())
	}
}

func (so *StatusObject) Error() string {
	return fmt.Sprintf("HTTP status %s", so.Status)
}

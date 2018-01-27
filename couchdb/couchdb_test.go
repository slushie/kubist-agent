package couchdb

import (
	"testing"
	"net/http/httptest"
	"net/http"
	"strings"
	"os"
	"github.com/magiconair/properties/assert"
	"fmt"
	"encoding/json"
	"encoding/base64"
)

type tearDownFunc func()

var (
	TestUrl            string
	TestRequest        *http.Request

	TestResponseStatus = http.StatusOK
	TestResponseBody   = Body{"ok": true}

	TestUsername = "test-username"
	TestPassword = "test-password"
	TestDatabase = "test-database"

	TestAuth = &Auth{TestUsername, TestPassword}
)

func setupAuthServer() tearDownFunc {
	srv := httptest.NewServer(
		http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
			var validCreds, givenCreds string

			TestRequest = req
			emptyCreds := TestUsername == "" && TestPassword == ""

			if !emptyCreds {
				validCreds = base64.StdEncoding.EncodeToString(
					[]byte(TestUsername + ":" + TestPassword),
				)
			}

			auth, hasAuth := req.Header["Authorization"]
			if hasAuth { _, givenCreds = parseAuth(auth[0]) }

			if (emptyCreds && hasAuth) || (givenCreds != validCreds){
				res.WriteHeader(http.StatusUnauthorized)
			} else {
				res.WriteHeader(TestResponseStatus)
			}

			body, err := json.Marshal(TestResponseBody)
			if err != nil {
				panic(err.Error())
			}

			res.Header().Set("Content-type", "application/json")
			res.Write(body)
		}),
	)

	TestUrl = srv.URL
	return srv.Close
}

func parseAuth(header string) (string, string) {
	k := strings.Split(header, " ")
	if len(k) < 2 {
		panic(fmt.Errorf("bad auth header %+v", header))
	}

	return strings.ToLower(k[0]), k[1]
}

func TestClientAuth(t *testing.T) {
	var testClientAuth = []struct {
		auth   *Auth
		status int
	}{
		{&Auth{}, 401},
		{&Auth{"bad-username", "bad-password"}, 401},
		{&Auth{TestUsername, TestPassword}, 200},
	}

	for _, tc := range testClientAuth {
		c, err := NewClient(TestUrl, tc.auth)
		if err != nil {
			t.Fatal(err)
		} else if status, err := c.Info(); err != nil {
			t.Fatal(err)
		} else {
			assert.Equal(t, status.StatusCode, tc.status,
				fmt.Sprintf("login with %+v", tc.auth))
		}
	}
}



func TestEmptyClientAuth(t *testing.T) {
	var testClientAuth = []struct {
		auth   *Auth
		status int
	}{
		{&Auth{"bad-username", "bad-password"}, 401},
		{&Auth{}, 200},
	}

	oldUsername, oldPassword := TestUsername, TestPassword
	TestUsername, TestPassword = "", ""
	defer func() {
		TestUsername, TestPassword = oldUsername, oldPassword
	}()

	for _, tc := range testClientAuth {
		c, err := NewClient(TestUrl, tc.auth)
		if err != nil {
			t.Fatal(err)
		} else if status, err := c.Info(); err != nil {
			t.Fatal(err)
		} else {
			assert.Equal(t, status.StatusCode, tc.status,
				fmt.Sprintf("login with %+v", tc.auth))
		}
	}
}

func TestClient_Database(t *testing.T) {
	var testDatabaseUrl = []struct {
		name string
		path string
	}{
		{"test-database", "/test-database"},
		{"test/database", "/test/database"},
	}

	c, err := NewClient(TestUrl, TestAuth)
	if err != nil {
		t.Fatal(err)
	}

	for _, td := range testDatabaseUrl {
		if db, ok := c.Database(td.name).(*Database); !ok {
			t.Errorf("%T is not *Database", db)
		} else if _, err := db.Post(nil); err != nil {
			t.Errorf("Post error: %s", err.Error())
		} else {
			assert.Equal(
				t,
				TestRequest.URL.Path,
				td.path,
			)
		}
	}
}

func TestDatabase_Changes(t *testing.T) {

}

func TestMain(tm *testing.M) {
	tearDown := setupAuthServer()
	defer tearDown()

	os.Exit(tm.Run())
}

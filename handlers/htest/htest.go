package htest

import (
	"context"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/jmoiron/sqlx"
	"github.com/teamwork/test"
	"github.com/teamwork/utils/jsonutil"
	"zgo.at/goatcounter"
	"zgo.at/zhttp/ctxkey"
)

var schema string

// Start a new test.
func Start(t *testing.T) (*sqlx.DB, func()) {
	t.Helper()

	db, err := sqlx.Connect("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}

	if schema == "" {
		schemaB, err := ioutil.ReadFile("../db/schema.sql")
		if err != nil {
			t.Fatal(err)
		}
		schema = string(schemaB)
	}

	_, err = db.Exec(schema) // TODO: also insert site?
	if err != nil {
		t.Fatal(err)
	}

	return db, func() { db.Close() }
}

// New request and recorder.
//
// TODO: use actual middleware.
func New(db *sqlx.DB, method, path string, body io.Reader) (*http.Request, *httptest.ResponseRecorder) {
	r := test.NewRequest(method, path, body)
	r = r.WithContext(context.WithValue(r.Context(), ctxkey.DB, db))
	r = r.WithContext(context.WithValue(r.Context(), ctxkey.Site, &goatcounter.Site{ID: 1}))
	r = r.WithContext(context.WithValue(r.Context(), ctxkey.User, &goatcounter.User{ID: 1, Site: 1}))
	return r, httptest.NewRecorder()
}

// Form converts to an "application/x-www-form-urlencoded" form.
//
// Use github.com/teamwork/test.Multipart for a multipart form.
//
// Note: this is primitive, but enough for now.
func Form(i interface{}) string {
	var m map[string]string
	jsonutil.MustUnmarshal(jsonutil.MustMarshal(i), &m)

	f := make(url.Values)
	for k, v := range m {
		f[k] = []string{v}
	}

	// TODO: null values are:
	// email=foo%40example.com&frequency=
	return f.Encode()
}

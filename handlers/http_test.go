package handlers

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/teamwork/test"
	"github.com/teamwork/utils/jsonutil"
	"zgo.at/goatcounter/handlers/htest"
	"zgo.at/zhttp"
)

type handlerTest struct {
	name         string
	setup        func(context.Context)
	handler      func(http.ResponseWriter, *http.Request) error
	body         interface{}
	wantErr      string
	wantJSONCode int
	wantFormCode int
	wantJSONBody string
	wantFormBody string
}

func runTest(
	t *testing.T,
	tt handlerTest,
	fun func(*testing.T, *httptest.ResponseRecorder, *http.Request),
) {
	t.Run(tt.name, func(t *testing.T) {
		t.Run("json", func(t *testing.T) {
			db, clean := htest.Start(t)
			defer clean()

			r, rr := htest.New(db, "POST", "/", bytes.NewReader(jsonutil.MustMarshal(tt.body)))
			if tt.setup != nil {
				tt.setup(r.Context())
			}

			zhttp.Wrap(tt.handler)(rr, r)
			test.Code(t, rr, tt.wantJSONCode)

			if !strings.Contains(rr.Body.String(), tt.wantJSONBody) {
				t.Errorf("wrong body\nwant: %s\ngot:  %s", tt.wantJSONBody, rr.Body.String())
			}

			if fun != nil {
				fun(t, rr, r)
			}
		})

		t.Run("form", func(t *testing.T) {
			db, clean := htest.Start(t)
			defer clean()

			form := htest.Form(tt.body)
			r, rr := htest.New(db, "POST", "/", strings.NewReader(form))
			r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			r.Header.Set("Content-Length", fmt.Sprintf("%d", len(form)))
			if tt.setup != nil {
				tt.setup(r.Context())
			}

			zhttp.Wrap(tt.handler)(rr, r)
			test.Code(t, rr, tt.wantFormCode)

			if !strings.Contains(rr.Body.String(), tt.wantFormBody) {
				t.Errorf("wrong body\nwant: %q\ngot:  %q", tt.wantFormBody, rr.Body.String())
			}

			if fun != nil {
				fun(t, rr, r)
			}
		})
	})
}

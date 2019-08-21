// Copyright © 2019 Martin Tournoij <martin@arp242.net>
// This file is part of GoatCounter and published under the terms of the AGPLv3,
// which can be found in the LICENSE file or at gnu.org/licenses/agpl.html

package htest

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"

	"github.com/teamwork/test"
	"github.com/teamwork/utils/jsonutil"
)

// New request and recorder.
//
// TODO: use actual middleware.
func New(ctx context.Context, method, path string, body io.Reader) (*http.Request, *httptest.ResponseRecorder) {
	return test.NewRequest(method, path, body).WithContext(ctx), httptest.NewRecorder()
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

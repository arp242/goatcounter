// Copyright © 2019 Martin Tournoij <martin@arp242.net>
// This file is part of GoatCounter and published under the terms of the EUPL
// v1.2, which can be found in the LICENSE file or at http://eupl12.zgo.at

package title

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestTitle(t *testing.T) {
	// Don't use the zhttputil.SafeClient in tests.
	client = &http.Client{}

	tests := []struct {
		html, title string
	}{
		{`<!DOCTYPE html><html><head><title>Test! </title></head></html>`, `Test!`},
		{`<html><head><title>Test! </title></head></html>`, `Test!`},
		{`<title>Test! </title><body><p>ads</p>`, `Test!`},
		{`<title>&lt;p&gt;asd&amp;</title><body><p>ads</p>`, `<p>asd&`},
		{`<title>€</title><body><p>ads</p>`, `€`},
		{`<title attr="val">€</title><body><p>ads</p>`, `€`},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				fmt.Fprint(w, tt.html)
			}))
			defer srv.Close()

			title, err := Get(srv.URL)
			if err != nil {
				t.Fatal(err)
			}
			if title != tt.title {
				t.Errorf("title wrong: %q", title)
			}
		})
	}
}

// Copyright © Martin Tournoij – This file is part of GoatCounter and published
// under the terms of a slightly modified EUPL v1.2 license, which can be found
// in the LICENSE file or at https://license.goatcounter.com

package handlers

import (
	"fmt"
	"net/http"

	"zgo.at/goatcounter/v2"
	"zgo.at/zhttp"
)

func MovedPermanently(w http.ResponseWriter, r *http.Request, path string) error {
	if path == "" || path[0] != '/' {
		panic(fmt.Sprintf("handlers.MovedPermantly: %q does not start with slash", path))
	}
	return zhttp.MovedPermanently(w, goatcounter.Config(r.Context()).BasePath+path)
}

func SeeOther(w http.ResponseWriter, r *http.Request, path string) error {
	if path == "" || path[0] != '/' {
		panic(fmt.Sprintf("handlers.SeeOther: %q does not start with slash", path))
	}
	return zhttp.SeeOther(w, goatcounter.Config(r.Context()).BasePath+path)
}

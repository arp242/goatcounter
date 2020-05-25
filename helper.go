// Copyright © 2019 Martin Tournoij <martin@arp242.net>
// This file is part of GoatCounter and published under the terms of the EUPL
// v1.2, which can be found in the LICENSE file or at http://eupl12.zgo.at

//go:generate go run gen.go

package goatcounter

import (
	"context"
	"fmt"
	"time"

	"zgo.at/goatcounter/cfg"
	"zgo.at/zdb"
	"zgo.at/zhttp"
	"zgo.at/zhttp/ctxkey"
)

// State column values.
const (
	StateActive  = "a"
	StateRequest = "r"
	StateDeleted = "d"
)

var States = []string{StateActive, StateRequest, StateDeleted}

// Now gets the current time in UTC; can be overwritten in tests.
var Now = func() time.Time { return time.Now().UTC() }

// WithSite adds the site to the context.
func WithSite(ctx context.Context, s *Site) context.Context {
	return context.WithValue(ctx, ctxkey.Site, s)
}

// GetSite gets the current site.
func GetSite(ctx context.Context) *Site {
	s, _ := ctx.Value(ctxkey.Site).(*Site)
	return s
}

// MustGetSite behaves as GetSite(), panicking if this fails.
func MustGetSite(ctx context.Context) *Site {
	s, ok := ctx.Value(ctxkey.Site).(*Site)
	if !ok {
		panic("MustGetSite: no site on context")
	}
	return s
}

// WithUser adds the site to the context.
func WithUser(ctx context.Context, u *User) context.Context {
	return context.WithValue(ctx, ctxkey.User, u)
}

// GetUser gets the currently logged in user.
func GetUser(ctx context.Context) *User {
	u, _ := ctx.Value(ctxkey.User).(*User)
	return u
}

// NewContext creates a new context with the all the request values set.
//
// Useful for tests, or for "removing" the timeout on the request context so it
// can be passed to background functions.
func NewContext(ctx context.Context) context.Context {
	n := zdb.With(context.Background(), zdb.MustGet(ctx))
	n = context.WithValue(n, ctxkey.User, GetUser(ctx))
	n = context.WithValue(n, ctxkey.Site, GetSite(ctx))
	return n
}

func EmailTemplate(tplname string, args interface{}) func() ([]byte, error) {
	return func() ([]byte, error) {
		return zhttp.ExecuteTpl(tplname, args)
		//buf := new(bytes.Buffer)
		//err := tpl.ExecuteTemplate(buf, tplname, args)
		//return buf.Bytes(), err
	}
}

func interval(days int) string {
	if cfg.PgSQL {
		return fmt.Sprintf(" now() - interval '%d days' ", days)
	}
	return fmt.Sprintf(" datetime(datetime(), '-%d days') ", days)
}

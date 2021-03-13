// Copyright © 2019 Martin Tournoij – This file is part of GoatCounter and
// published under the terms of a slightly modified EUPL v1.2 license, which can
// be found in the LICENSE file or at https://license.goatcounter.com

package goatcounter

import (
	"context"
	"time"

	"zgo.at/zcache"
	"zgo.at/zdb"
	"zgo.at/zhttp/ctxkey"
)

var Version = ""

var (
	keyCacheSites      = &struct{ n string }{""}
	keyCacheUA         = &struct{ n string }{""}
	keyCacheBrowsers   = &struct{ n string }{""}
	keyCacheSystems    = &struct{ n string }{""}
	keyCachePaths      = &struct{ n string }{""}
	keyCacheLoc        = &struct{ n string }{""}
	keyChangedTitles   = &struct{ n string }{""}
	keyCacheSitesProxy = &struct{ n string }{""}

	keyConfig = &struct{ n string }{""}
)

type GlobalConfig struct {
	Domain         string
	DomainStatic   string
	DomainCount    string
	URLStatic      string
	Plan           string
	Dev            bool
	Version        string
	GoatcounterCom bool
	Serve          bool
	Port           string
	EmailFrom      string
	BcryptMinCost  bool
}

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

// CopyContextValues creates a new context with the all the request values set.
//
// Useful for tests, or for "removing" the timeout on the request context so it
// can be passed to background functions.
func CopyContextValues(ctx context.Context) context.Context {
	n := zdb.WithDB(context.Background(), zdb.MustGetDB(ctx))

	if c := ctx.Value(keyCacheSites); c != nil {
		n = context.WithValue(n, keyCacheSites, c.(*zcache.Cache))
	}
	if c := ctx.Value(keyCacheUA); c != nil {
		n = context.WithValue(n, keyCacheUA, c.(*zcache.Cache))
	}
	if c := ctx.Value(keyCacheBrowsers); c != nil {
		n = context.WithValue(n, keyCacheBrowsers, c.(*zcache.Cache))
	}
	if c := ctx.Value(keyCacheSystems); c != nil {
		n = context.WithValue(n, keyCacheSystems, c.(*zcache.Cache))
	}
	if c := ctx.Value(keyCachePaths); c != nil {
		n = context.WithValue(n, keyCachePaths, c.(*zcache.Cache))
	}
	if c := ctx.Value(keyCacheLoc); c != nil {
		n = context.WithValue(n, keyCacheLoc, c.(*zcache.Cache))
	}
	if c := ctx.Value(keyChangedTitles); c != nil {
		n = context.WithValue(n, keyChangedTitles, c.(*zcache.Cache))
	}
	if c := ctx.Value(keyCacheSitesProxy); c != nil {
		n = context.WithValue(n, keyCacheSitesProxy, c.(*zcache.Proxy))
	}
	if c := Config(ctx); c != nil {
		n = context.WithValue(n, keyConfig, c)
	}
	if s := GetSite(ctx); s != nil {
		n = context.WithValue(n, ctxkey.Site, s)
	}
	if u := GetUser(ctx); u != nil {
		n = context.WithValue(n, ctxkey.User, u)
	}
	return n
}

// NewContext creates a new context with all values set.
func NewContext(db zdb.DB) context.Context {
	ctx := zdb.WithDB(context.Background(), db)
	ctx = NewCache(ctx)
	ctx = NewConfig(ctx)
	return ctx
}

func NewCache(ctx context.Context) context.Context {
	s := zcache.New(24*time.Hour, 1*time.Hour)
	ctx = context.WithValue(ctx, keyCacheSites, s)
	ctx = context.WithValue(ctx, keyCacheSitesProxy, zcache.NewProxy(s))

	ctx = context.WithValue(ctx, keyCacheUA, zcache.New(1*time.Hour, 5*time.Minute))
	ctx = context.WithValue(ctx, keyCacheBrowsers, zcache.New(1*time.Hour, 5*time.Minute))
	ctx = context.WithValue(ctx, keyCacheSystems, zcache.New(1*time.Hour, 5*time.Minute))
	ctx = context.WithValue(ctx, keyCachePaths, zcache.New(1*time.Hour, 5*time.Minute))
	ctx = context.WithValue(ctx, keyCacheLoc, zcache.New(zcache.NoExpiration, zcache.NoExpiration))
	ctx = context.WithValue(ctx, keyChangedTitles, zcache.New(48*time.Hour, 1*time.Hour))
	return ctx
}

func NewConfig(ctx context.Context) context.Context {
	return context.WithValue(ctx, keyConfig, &GlobalConfig{})
}

func Config(ctx context.Context) *GlobalConfig {
	return ctx.Value(keyConfig).(*GlobalConfig)
}

func cacheSites(ctx context.Context) *zcache.Cache { return ctx.Value(keyCacheSites).(*zcache.Cache) }
func cacheUA(ctx context.Context) *zcache.Cache    { return ctx.Value(keyCacheUA).(*zcache.Cache) }
func cacheBrowsers(ctx context.Context) *zcache.Cache {
	return ctx.Value(keyCacheBrowsers).(*zcache.Cache)
}
func cacheSystems(ctx context.Context) *zcache.Cache {
	return ctx.Value(keyCacheSystems).(*zcache.Cache)
}
func cachePaths(ctx context.Context) *zcache.Cache { return ctx.Value(keyCachePaths).(*zcache.Cache) }
func cacheLoc(ctx context.Context) *zcache.Cache   { return ctx.Value(keyCacheLoc).(*zcache.Cache) }
func cacheChangedTitles(ctx context.Context) *zcache.Cache {
	return ctx.Value(keyChangedTitles).(*zcache.Cache)
}
func cacheSitesHost(ctx context.Context) *zcache.Proxy {
	return ctx.Value(keyCacheSitesProxy).(*zcache.Proxy)
}

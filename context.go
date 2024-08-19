// Copyright © Martin Tournoij – This file is part of GoatCounter and published
// under the terms of a slightly modified EUPL v1.2 license, which can be found
// in the LICENSE file or at https://license.goatcounter.com

package goatcounter

import (
	"context"
	"fmt"
	"time"

	"zgo.at/z18n"
	"zgo.at/zcache"
	"zgo.at/zdb"
	"zgo.at/zhttp/ctxkey"
)

// Version of GoatCounter; set at compile-time with:
//
//   -ldflags="-X zgo.at/goatcounter/v2.Version=…"

var Version = "dev"

var (
	keyCacheSites      = &struct{ n string }{""}
	keyCacheUA         = &struct{ n string }{""}
	keyCacheBrowsers   = &struct{ n string }{""}
	keyCacheSystems    = &struct{ n string }{""}
	keyCachePaths      = &struct{ n string }{""}
	keyCacheRefs       = &struct{ n string }{""}
	keyCacheSizes      = &struct{ n string }{""}
	keyCacheLoc        = &struct{ n string }{""}
	keyCacheCampaigns  = &struct{ n string }{""}
	keyChangedTitles   = &struct{ n string }{""}
	keyCacheSitesProxy = &struct{ n string }{""}
	keyCacheI18n       = &struct{ n string }{""}

	keyConfig = &struct{ n string }{""}
)

type GlobalConfig struct {
	Domain         string
	DomainStatic   string
	DomainCount    string
	BasePath       string
	URLStatic      string
	Dev            bool
	GoatcounterCom bool
	Port           string
	Websocket      bool
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

// GetAccount gets this site's "account site" on which the users, etc. are
// stored.
func GetAccount(ctx context.Context) (*Site, error) {
	s := MustGetSite(ctx)
	if s.Parent == nil {
		return s, nil
	}

	var account Site
	err := account.ByID(ctx, *s.Parent)
	if err != nil {
		return nil, fmt.Errorf("GetAccount: %w", err)
	}
	return &account, nil
}

func MustGetAccount(ctx context.Context) *Site {
	a, err := GetAccount(ctx)
	if err != nil {
		panic(err)
	}
	return a
}

// WithUser adds the site to the context.
func WithUser(ctx context.Context, u *User) context.Context {
	return context.WithValue(ctx, ctxkey.User, u)
}

// GetUser gets the currently logged in user.
func GetUser(ctx context.Context) *User {
	u, _ := ctx.Value(ctxkey.User).(*User)
	if u == nil || u.ID == 0 {
		s := GetSite(ctx)
		if s != nil {
			return &User{Settings: s.UserDefaults}
		}
	}
	return u
}

// MustGetUser behaves as GetUser(), panicking if this fails.
func MustGetUser(ctx context.Context) *User {
	u := GetUser(ctx)
	if u == nil {
		panic("MustGetUser: no user on context")
	}
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
	if c := ctx.Value(keyCacheRefs); c != nil {
		n = context.WithValue(n, keyCacheRefs, c.(*zcache.Cache))
	}
	if c := ctx.Value(keyCacheSizes); c != nil {
		n = context.WithValue(n, keyCacheSizes, c.(*zcache.Cache))
	}
	if c := ctx.Value(keyCacheLoc); c != nil {
		n = context.WithValue(n, keyCacheLoc, c.(*zcache.Cache))
	}
	if c := ctx.Value(keyCacheCampaigns); c != nil {
		n = context.WithValue(n, keyCacheCampaigns, c.(*zcache.Cache))
	}
	if c := ctx.Value(keyCacheI18n); c != nil {
		n = context.WithValue(n, keyCacheI18n, c.(*zcache.Cache))
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
	if l := z18n.Get(ctx); l != nil {
		n = z18n.With(n, l)
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
	ctx = context.WithValue(ctx, keyCacheRefs, zcache.New(1*time.Hour, 5*time.Minute))
	ctx = context.WithValue(ctx, keyCacheSizes, zcache.New(1*time.Hour, 5*time.Minute))
	ctx = context.WithValue(ctx, keyCacheLoc, zcache.New(zcache.NoExpiration, zcache.NoExpiration))
	ctx = context.WithValue(ctx, keyCacheCampaigns, zcache.New(24*time.Hour, 15*time.Minute))
	ctx = context.WithValue(ctx, keyCacheI18n, zcache.New(zcache.NoExpiration, zcache.NoExpiration))
	ctx = context.WithValue(ctx, keyChangedTitles, zcache.New(48*time.Hour, 1*time.Hour))
	return ctx
}

func NewConfig(ctx context.Context) context.Context {
	return context.WithValue(ctx, keyConfig, &GlobalConfig{})
}

func Config(ctx context.Context) *GlobalConfig {
	if c := ctx.Value(keyConfig); c != nil {
		return c.(*GlobalConfig)
	}
	return &GlobalConfig{}
}

func cacheSites(ctx context.Context) *zcache.Cache {
	if c := ctx.Value(keyCacheSites); c != nil {
		return c.(*zcache.Cache)
	}
	return zcache.New(0, 0)
}
func cacheUA(ctx context.Context) *zcache.Cache {
	if c := ctx.Value(keyCacheUA); c != nil {
		return c.(*zcache.Cache)
	}
	return zcache.New(0, 0)
}
func cacheBrowsers(ctx context.Context) *zcache.Cache {
	if c := ctx.Value(keyCacheBrowsers); c != nil {
		return c.(*zcache.Cache)
	}
	return zcache.New(0, 0)
}
func cacheSystems(ctx context.Context) *zcache.Cache {
	if c := ctx.Value(keyCacheSystems); c != nil {
		return c.(*zcache.Cache)
	}
	return zcache.New(0, 0)
}
func cachePaths(ctx context.Context) *zcache.Cache {
	if c := ctx.Value(keyCachePaths); c != nil {
		return c.(*zcache.Cache)
	}
	return zcache.New(0, 0)
}
func cacheRefs(ctx context.Context) *zcache.Cache {
	if c := ctx.Value(keyCacheRefs); c != nil {
		return c.(*zcache.Cache)
	}
	return zcache.New(0, 0)
}
func cacheSizes(ctx context.Context) *zcache.Cache {
	if c := ctx.Value(keyCacheSizes); c != nil {
		return c.(*zcache.Cache)
	}
	return zcache.New(0, 0)
}
func cacheLoc(ctx context.Context) *zcache.Cache {
	if c := ctx.Value(keyCacheLoc); c != nil {
		return c.(*zcache.Cache)
	}
	return zcache.New(0, 0)
}
func cacheCampaigns(ctx context.Context) *zcache.Cache {
	if c := ctx.Value(keyCacheCampaigns); c != nil {
		return c.(*zcache.Cache)
	}
	return zcache.New(0, 0)
}
func cacheI18n(ctx context.Context) *zcache.Cache {
	if c := ctx.Value(keyCacheI18n); c != nil {
		return c.(*zcache.Cache)
	}
	return zcache.New(0, 0)
}
func cacheChangedTitles(ctx context.Context) *zcache.Cache {
	if c := ctx.Value(keyChangedTitles); c != nil {
		return c.(*zcache.Cache)
	}
	return zcache.New(0, 0)
}
func cacheSitesHost(ctx context.Context) *zcache.Proxy {
	if c := ctx.Value(keyCacheSitesProxy); c != nil {
		return c.(*zcache.Proxy)
	}
	return zcache.NewProxy(zcache.New(0, 0))
}

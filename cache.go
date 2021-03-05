// Copyright © 2019 Martin Tournoij – This file is part of GoatCounter and
// published under the terms of a slightly modified EUPL v1.2 license, which can
// be found in the LICENSE file or at https://license.goatcounter.com

package goatcounter

import (
	"context"
	"time"

	"zgo.at/zcache"
)

var (
	keyCacheSites      = &struct{ n string }{""}
	keyCacheUA         = &struct{ n string }{""}
	keyCacheBrowsers   = &struct{ n string }{""}
	keyCacheSystems    = &struct{ n string }{""}
	keyCachePaths      = &struct{ n string }{""}
	keyCacheLoc        = &struct{ n string }{""}
	keyChangedTitles   = &struct{ n string }{""}
	keyCacheSitesProxy = &struct{ n string }{""}
)

func NewCache(ctx context.Context) context.Context {
	s := zcache.New(24*time.Hour, 1*time.Hour)
	sh := zcache.NewProxy(s)
	ctx = context.WithValue(ctx, keyCacheSites, s)
	ctx = context.WithValue(ctx, keyCacheSitesProxy, &sh)

	ctx = context.WithValue(ctx, keyCacheUA, zcache.New(1*time.Hour, 5*time.Minute))
	ctx = context.WithValue(ctx, keyCacheBrowsers, zcache.New(1*time.Hour, 5*time.Minute))
	ctx = context.WithValue(ctx, keyCacheSystems, zcache.New(1*time.Hour, 5*time.Minute))
	ctx = context.WithValue(ctx, keyCachePaths, zcache.New(1*time.Hour, 5*time.Minute))
	ctx = context.WithValue(ctx, keyCacheLoc, zcache.New(zcache.NoExpiration, zcache.NoExpiration))
	ctx = context.WithValue(ctx, keyChangedTitles, zcache.New(48*time.Hour, 1*time.Hour))
	return ctx
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

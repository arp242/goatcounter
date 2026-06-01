package goatcounter

import (
	"context"

	"zgo.at/zcache/v2"
	"zgo.at/zstd/zruntime"
)

func ListCache(ctx context.Context) map[string]struct{ Num, Size int64 } {
	caches := map[string]interface {
		ItemsAny() map[any]zcache.Item[any]
	}{
		"sites":          cacheSites(ctx),
		"ua":             cacheUA(ctx),
		"browsers":       cacheBrowsers(ctx),
		"systems":        cacheSystems(ctx),
		"paths":          cachePaths(ctx),
		"refs":           cacheRefs(ctx),
		"loc":            cacheLoc(ctx),
		"campaigns":      cacheCampaigns(ctx),
		"changed_titles": cacheChangedTitles(ctx),
		//"loader":         handler.loader.conns,
	}

	c := make(map[string]struct{ Num, Size int64 })
	for name, f := range caches {
		var (
			content = f.ItemsAny()
			s       = zruntime.SizeOf(content)
		)
		for _, v := range content {
			s += c[name].Size + zruntime.SizeOf(v.Object)
		}
		c[name] = struct {
			Num, Size int64
		}{int64(len(content)), s / 1024}
	}

	{
		var (
			name    = "sites_host"
			content = cacheSitesHost(ctx).Items()
			s       = zruntime.SizeOf(content)
		)
		for _, v := range content {
			s += c[name].Size + zruntime.SizeOf(v)
		}
		c[name] = struct{ Num, Size int64 }{int64(len(content)), s / 1024}
	}
	return c
}

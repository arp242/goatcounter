package goatcounter

import (
	"context"
	"fmt"

	"zgo.at/zcache/v2"
	"zgo.at/zstd/zjson"
	"zgo.at/zstd/zruntime"
)

func ListCache(ctx context.Context) map[string]struct {
	Size  int64
	Items map[string]string
} {
	c := make(map[string]struct {
		Size  int64
		Items map[string]string
	})

	caches := map[string]interface {
		ItemsAny() map[any]zcache.Item[any]
	}{
		"sites":          cacheSites(ctx),
		"ua":             cacheUA(ctx),
		"browsers":       cacheBrowsers(ctx),
		"systems":        cacheSystems(ctx),
		"paths":          cachePaths(ctx),
		"loc":            cacheLoc(ctx),
		"changed_titles": cacheChangedTitles(ctx),
		//"loader":         handler.loader.conns,
	}

	for name, f := range caches {
		var (
			content = f.ItemsAny()
			s       = zruntime.SizeOf(content)
			items   = make(map[string]string)
		)
		for k, v := range content {
			items[fmt.Sprintf("%v", k)] = fmt.Sprintf("%s\n", zjson.MustMarshalIndent(v.Object, "", "  "))
			s += c[name].Size + zruntime.SizeOf(v.Object)
		}
		c[name] = struct {
			Size  int64
			Items map[string]string
		}{s / 1024, items}
	}

	{
		var (
			name    = "sites_host"
			content = cacheSitesHost(ctx).Items()
			s       = zruntime.SizeOf(content)
			items   = make(map[string]string)
		)
		for k, v := range content {
			items[k] = fmt.Sprintf("%d", v)
			s += c[name].Size + zruntime.SizeOf(v)
		}
		c[name] = struct {
			Size  int64
			Items map[string]string
		}{s / 1024, items}
	}
	return c
}

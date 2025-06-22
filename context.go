package goatcounter

import (
	"context"
	"fmt"
	"runtime/debug"
	"time"

	"zgo.at/goatcounter/v2/pkg/geo"
	"zgo.at/z18n"
	"zgo.at/zcache/v2"
	"zgo.at/zdb"
	"zgo.at/zhttp/ctxkey"
)

// Version of GoatCounter; set at compile-time with:
//
//	-ldflags="-X zgo.at/goatcounter/v2.Version=…"
var Version = "dev"

func getCommit() (string, time.Time, bool) {
	var (
		rev     string
		last    time.Time
		dirty   bool
		info, _ = debug.ReadBuildInfo()
	)
	for _, kv := range info.Settings {
		switch kv.Key {
		case "vcs.revision":
			rev = kv.Value
		case "vcs.time":
			last, _ = time.Parse(time.RFC3339, kv.Value)
		case "vcs.modified":
			dirty = kv.Value == "true"
		}
	}
	return rev, last, dirty
}

func init() {
	if Version == "" || Version == "dev" {
		// Only calculate the version if not explicitly overridden with:
		//	-ldflags="-X zgo.at/goatcounter/v2.Version=$tag"
		// which is done for release builds.
		if rev, last, dirty := getCommit(); rev != "" {
			Version = rev[:12] + "_" + last.Format("2006-01-02T15:04:05Z0700")
			if dirty {
				Version += "-dev"
			}
		}
	}
}

var (
	keyCacheSites      = &struct{ n string }{""}
	keyCacheUA         = &struct{ n string }{""}
	keyCacheBrowsers   = &struct{ n string }{""}
	keyCacheSystems    = &struct{ n string }{""}
	keyCachePaths      = &struct{ n string }{""}
	keyCacheRefs       = &struct{ n string }{""}
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
		n = context.WithValue(n, keyCacheSites, c.(*zcache.Cache[SiteID, *Site]))
	}
	if c := ctx.Value(keyCacheUA); c != nil {
		n = context.WithValue(n, keyCacheUA, c.(*zcache.Cache[string, UserAgent]))
	}
	if c := ctx.Value(keyCacheBrowsers); c != nil {
		n = context.WithValue(n, keyCacheBrowsers, c.(*zcache.Cache[string, Browser]))
	}
	if c := ctx.Value(keyCacheSystems); c != nil {
		n = context.WithValue(n, keyCacheSystems, c.(*zcache.Cache[string, System]))
	}
	if c := ctx.Value(keyCachePaths); c != nil {
		n = context.WithValue(n, keyCachePaths, c.(*zcache.Cache[string, Path]))
	}
	if c := ctx.Value(keyCacheRefs); c != nil {
		n = context.WithValue(n, keyCacheRefs, c.(*zcache.Cache[string, Ref]))
	}
	if c := ctx.Value(keyCacheLoc); c != nil {
		n = context.WithValue(n, keyCacheLoc, c.(*zcache.Cache[string, *Location]))
	}
	if c := ctx.Value(keyCacheCampaigns); c != nil {
		n = context.WithValue(n, keyCacheCampaigns, c.(*zcache.Cache[string, *Campaign]))
	}
	if c := ctx.Value(keyCacheI18n); c != nil {
		n = context.WithValue(n, keyCacheI18n, c.(*zcache.Cache[string, *OverrideTranslations]))
	}
	if c := ctx.Value(keyChangedTitles); c != nil {
		n = context.WithValue(n, keyChangedTitles, c.(*zcache.Cache[string, []string]))
	}
	if c := ctx.Value(keyCacheSitesProxy); c != nil {
		n = context.WithValue(n, keyCacheSitesProxy, c.(*zcache.Proxy[string, SiteID, *Site]))
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
	if g := geo.Get(ctx); g != nil {
		n = geo.With(n, g)
	}
	return n
}

// NewContext creates a new context with all values set.
func NewContext(ctx context.Context, db zdb.DB) context.Context {
	n := zdb.WithDB(context.Background(), db)
	n = geo.With(n, geo.Get(ctx))
	n = NewCache(n)
	n = NewConfig(n)
	return n
}

func NewCache(ctx context.Context) context.Context {
	s := zcache.New[SiteID, *Site](24*time.Hour, 1*time.Hour)
	ctx = context.WithValue(ctx, keyCacheSites, s)
	ctx = context.WithValue(ctx, keyCacheSitesProxy, zcache.NewProxy[string, SiteID, *Site](s))

	ctx = context.WithValue(ctx, keyCacheUA, zcache.New[string, UserAgent](1*time.Hour, 5*time.Minute))
	ctx = context.WithValue(ctx, keyCacheBrowsers, zcache.New[string, Browser](1*time.Hour, 5*time.Minute))
	ctx = context.WithValue(ctx, keyCacheSystems, zcache.New[string, System](1*time.Hour, 5*time.Minute))
	ctx = context.WithValue(ctx, keyCachePaths, zcache.New[string, Path](1*time.Hour, 5*time.Minute))
	ctx = context.WithValue(ctx, keyCacheRefs, zcache.New[string, Ref](1*time.Hour, 5*time.Minute))
	ctx = context.WithValue(ctx, keyCacheLoc, zcache.New[string, *Location](zcache.NoExpiration, zcache.NoExpiration))
	ctx = context.WithValue(ctx, keyCacheCampaigns, zcache.New[string, *Campaign](24*time.Hour, 15*time.Minute))
	ctx = context.WithValue(ctx, keyCacheI18n, zcache.New[string, *OverrideTranslations](zcache.NoExpiration, zcache.NoExpiration))
	ctx = context.WithValue(ctx, keyChangedTitles, zcache.New[string, []string](48*time.Hour, 1*time.Hour))
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

func cacheSites(ctx context.Context) *zcache.Cache[SiteID, *Site] {
	if c := ctx.Value(keyCacheSites); c != nil {
		return c.(*zcache.Cache[SiteID, *Site])
	}
	return zcache.New[SiteID, *Site](0, 0)
}
func cacheUA(ctx context.Context) *zcache.Cache[string, UserAgent] {
	if c := ctx.Value(keyCacheUA); c != nil {
		return c.(*zcache.Cache[string, UserAgent])
	}
	return zcache.New[string, UserAgent](0, 0)
}
func cacheBrowsers(ctx context.Context) *zcache.Cache[string, Browser] {
	if c := ctx.Value(keyCacheBrowsers); c != nil {
		return c.(*zcache.Cache[string, Browser])
	}
	return zcache.New[string, Browser](0, 0)
}
func cacheSystems(ctx context.Context) *zcache.Cache[string, System] {
	if c := ctx.Value(keyCacheSystems); c != nil {
		return c.(*zcache.Cache[string, System])
	}
	return zcache.New[string, System](0, 0)
}
func cachePaths(ctx context.Context) *zcache.Cache[string, Path] {
	if c := ctx.Value(keyCachePaths); c != nil {
		return c.(*zcache.Cache[string, Path])
	}
	return zcache.New[string, Path](0, 0)
}
func cacheRefs(ctx context.Context) *zcache.Cache[string, Ref] {
	if c := ctx.Value(keyCacheRefs); c != nil {
		return c.(*zcache.Cache[string, Ref])
	}
	return zcache.New[string, Ref](0, 0)
}
func cacheLoc(ctx context.Context) *zcache.Cache[string, *Location] {
	if c := ctx.Value(keyCacheLoc); c != nil {
		return c.(*zcache.Cache[string, *Location])
	}
	return zcache.New[string, *Location](0, 0)
}
func cacheCampaigns(ctx context.Context) *zcache.Cache[string, *Campaign] {
	if c := ctx.Value(keyCacheCampaigns); c != nil {
		return c.(*zcache.Cache[string, *Campaign])
	}
	return zcache.New[string, *Campaign](0, 0)
}
func cacheI18n(ctx context.Context) *zcache.Cache[string, *OverrideTranslations] {
	if c := ctx.Value(keyCacheI18n); c != nil {
		return c.(*zcache.Cache[string, *OverrideTranslations])
	}
	return zcache.New[string, *OverrideTranslations](0, 0)
}
func cacheChangedTitles(ctx context.Context) *zcache.Cache[string, []string] {
	if c := ctx.Value(keyChangedTitles); c != nil {
		return c.(*zcache.Cache[string, []string])
	}
	return zcache.New[string, []string](0, 0)
}
func cacheSitesHost(ctx context.Context) *zcache.Proxy[string, SiteID, *Site] {
	if c := ctx.Value(keyCacheSitesProxy); c != nil {
		return c.(*zcache.Proxy[string, SiteID, *Site])
	}
	return zcache.NewProxy[string, SiteID, *Site](zcache.New[SiteID, *Site](0, 0))
}

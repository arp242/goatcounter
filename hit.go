package goatcounter

import (
	"context"
	"net/url"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/teamwork/utils/jsonutil"
	"github.com/teamwork/validate"
	"zgo.at/zlog"
)

type Hit struct {
	Site int64 `db:"site" json:"-"`

	Path        string    `db:"path" json:"p,omitempty"`
	Ref         string    `db:"ref" json:"r,omitempty"`
	RefParams   *string   `db:"ref_params" json:"ref_params,omitempty"`
	RefOriginal *string   `db:"ref_original" json:"ref_original,omitempty"`
	CreatedAt   time.Time `db:"created_at" json:"-"`

	refURL *url.URL `db:"-" json:"-"`
}

var groups = map[string]string{
	"news.ycombinator.com":               "Hacker News",
	"hn.algolia.com":                     "Hacker News",
	"hckrnews.com":                       "Hacker News",
	"hn.premii.com":                      "Hacker News",
	"com.stefandekanski.hackernews.free": "Hacker News",

	"mail.google.com":       "Gmail",
	"com.google.android.gm": "Gmail",

	"com.google.android.googlequicksearchbox":                      "Google",
	"com.google.android.googlequicksearchbox/https/www.google.com": "Google",

	"com.andrewshu.android.reddit":       "www.reddit.com",
	"com.laurencedawson.reddit_sync":     "www.reddit.com",
	"com.laurencedawson.reddit_sync.dev": "www.reddit.com",
	"com.laurencedawson.reddit_sync.pro": "www.reddit.com",

	"m.facebook.com":  "www.facebook.com",
	"l.facebook.com":  "www.facebook.com",
	"lm.facebook.com": "www.facebook.com",

	"org.telegram.messenger": "Telegram Messenger",

	"com.Slack": "Slack Chat",
}

var hostAlias = map[string]string{
	"en.m.wikipedia.org": "en.wikipedia.org",
	"old.reddit.com":     "www.reddit.com",
	"m.facebook.com":     "www.facebook.com",
	"m.habr.com":         "habr.com",
}

func cleanURL(ref string, refURL *url.URL) (string, *string, bool) {
	// I'm not sure where these links are generated, but there are *a lot* of
	// them.
	if refURL.Host == "link.oreilly.com" {
		return "https://link.oreilly.com", nil, true
	}

	changed := false

	// Normalize some hosts.
	if a, ok := hostAlias[refURL.Host]; ok {
		changed = true
		refURL.Host = a
	}

	// Group based on URL.
	if strings.HasPrefix(refURL.Host, "www.google.") {
		return "Google", nil, true
	}
	if g, ok := groups[refURL.Host]; ok {
		return g, nil, true
	}

	// Clean query parameters.
	i := strings.Index(ref, "?")
	if i == -1 {
		// No parameters so no work.
		return refURL.String(), nil, changed
	}
	eq := ref[i+1:]
	ref = ref[:i]

	// Twitter's t.co links add this.
	if refURL.Host == "t.co" && eq == "amp=1" {
		return ref, nil, false
	}

	q := refURL.Query()
	refURL.RawQuery = ""
	start := len(q)

	// Google analytics tracking parameters.
	q.Del("utm_source")
	q.Del("utm_medium")
	q.Del("utm_campaign")
	q.Del("utm_term")

	if len(q) == 0 {
		return refURL.String(), nil, changed || len(q) != start
	}
	eq = q.Encode()
	return refURL.String(), &eq, changed || len(q) != start
}

// Defaults sets fields to default values, unless they're already set.
func (h *Hit) Defaults(ctx context.Context) {
	// site := MustGetSite(ctx)
	// h.Site = site.ID

	if h.CreatedAt.IsZero() {
		h.CreatedAt = time.Now().UTC()
	}

	if h.Ref != "" && h.refURL != nil {
		var store bool
		r := h.Ref
		h.Ref, h.RefParams, store = cleanURL(h.Ref, h.refURL)
		if store {
			h.RefOriginal = &r
		}
	}

	h.Ref = strings.TrimRight(h.Ref, "/")
	h.Path = "/" + strings.Trim(h.Path, "/")
}

// Validate the object.
func (h *Hit) Validate(ctx context.Context) error {
	v := validate.New()

	v.Required("site", h.Site)
	v.Required("path", h.Path)

	return v.ErrorOrNil()
}

// Insert a new row.
func (h *Hit) Insert(ctx context.Context) error {
	var err error
	h.refURL, err = url.Parse(h.Ref)
	if err != nil {
		zlog.Fields(zlog.F{"ref": h.Ref}).Errorf("could not parse ref: %s", err)
	}

	// Ignore spammers.
	if _, ok := blacklist[h.refURL.Host]; ok {
		return nil
	}

	h.Defaults(ctx)
	err = h.Validate(ctx)
	if err != nil {
		return err
	}

	db := MustGetDB(ctx)
	_, err = db.ExecContext(ctx, `insert into hits (site, path, ref, ref_params, ref_original)
		values ($1, $2, $3, $4, $5)`, h.Site, h.Path, h.Ref, h.RefParams, h.RefOriginal)
	return errors.Wrap(err, "Site.Insert")
}

type HitStat struct {
	Day  string
	Days [][]int
}

type HitStats []struct {
	Count int    `db:"count"`
	Max   int    `db:"-"`
	Path  string `db:"path"`
	Stats []HitStat
}

func (h *HitStats) List(ctx context.Context, start, end time.Time) error {
	db := MustGetDB(ctx)
	site := MustGetSite(ctx)

	err := db.SelectContext(ctx, h, `
		select path, count(path) as count
		from hits
		where
			site=$1 and
			date(created_at) >= $2 and
			date(created_at) <= $3
		group by path
		order by count desc
		limit 500`, site.ID, start.Format("2006-01-02"), end.Format("2006-01-02"))
	if err != nil {
		return errors.Wrap(err, "HitStats.List")
	}

	// Add stats
	type stats struct {
		Path  string    `json:"path"`
		Day   time.Time `json:"day"`
		Stats []byte    `json:"stats"`
	}
	var st []stats
	err = db.SelectContext(ctx, &st, `
		select path, day, stats
		from hit_stats
		where
			site=$1 and
			kind="h" and
			date(day) >= $2 and
			date(day) <= $3
		order by day asc
		`, site.ID, start.Format("2006-01-02"), end.Format("2006-01-02"))
	if err != nil {
		return errors.Wrap(err, "HitStats.List")
	}

	// TODO: meh...
	hh := *h
	for i := range hh {
		hh[i].Stats = make([]HitStat, len(st))

		for j, s := range st {
			if s.Path == hh[i].Path {
				var x [][]int
				jsonutil.MustUnmarshal(s.Stats, &x)
				hh[i].Stats[j] = HitStat{Day: s.Day.Format("2006-01-02"), Days: x}

				// Get max.
				// TODO: should maybe store this?
				for j := range x {
					if x[j][1] > hh[i].Max {
						hh[i].Max = x[j][1]
					}
				}
			}
		}

		if hh[i].Max < 10 {
			hh[i].Max = 10
		}
	}

	return nil
}

// ListRefs lists all references for a path.
func (h *HitStats) ListRefs(ctx context.Context, path string, start, end time.Time) error {
	db := MustGetDB(ctx)
	site := MustGetSite(ctx)

	err := db.SelectContext(ctx, h, `
		select ref as path, count(ref) as count
		from hits
		where
			site=$1 and
			path=$2S and
			date(created_at) >= $3 and
			date(created_at) <= $4
		group by ref
		order by count(*) desc
		limit 50
	`, site.ID, path, start.Format("2006-01-02"), end.Format("2006-01-02"))

	return errors.Wrap(err, "RefStats.ListRefs")
}

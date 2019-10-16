// Copyright © 2019 Martin Tournoij <martin@arp242.net>
// This file is part of GoatCounter and published under the terms of the AGPLv3,
// which can be found in the LICENSE file or at gnu.org/licenses/agpl.html

package goatcounter

import (
	"context"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/pkg/errors"
	"zgo.at/utils/jsonutil"
	"zgo.at/utils/sqlutil"
	"zgo.at/validate"
	"zgo.at/zdb"
	"zgo.at/zlog"
)

func ptr(s string) *string { return &s }

// ref_scheme column
var (
	RefSchemeHTTP      = ptr("h")
	RefSchemeOther     = ptr("o")
	RefSchemeGenerated = ptr("g")
)

type Hit struct {
	Site int64 `db:"site" json:"-"`

	Path        string            `db:"path" json:"p,omitempty"`
	Ref         string            `db:"ref" json:"r,omitempty"`
	RefParams   *string           `db:"ref_params" json:"-"`
	RefOriginal *string           `db:"ref_original" json:"-"`
	RefScheme   *string           `db:"ref_scheme" json:"-"`
	Browser     string            `db:"browser" json:"-"`
	Size        sqlutil.FloatList `db:"size" json:"s"`
	CreatedAt   time.Time         `db:"created_at" json:"-"`

	refURL *url.URL `db:"-"`
}

var groups = map[string]string{
	// HN has <meta name="referrer" content="origin"> so we only get the domain.
	"news.ycombinator.com":               "Hacker News",
	"hn.algolia.com":                     "Hacker News",
	"hckrnews.com":                       "Hacker News",
	"hn.premii.com":                      "Hacker News",
	"com.stefandekanski.hackernews.free": "Hacker News",
	"io.github.hidroh.materialistic":     "Hacker News",
	"hackerweb.app":                      "Hacker News",
	"www.daemonology.net/hn-daily":       "Hacker News",
	"quiethn.com":                        "Hacker News",
	// http://www.elegantreader.com/item/17358103
	// https://www.daemonology.net/hn-daily/2019-05.html

	"mail.google.com":       "Email",
	"com.google.android.gm": "Email",
	"mail.yahoo.com":        "Email",
	//  https://mailchi.mp

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
	"m.facebook.com":     "www.facebook.com",
	"m.habr.com":         "habr.com",
	"old.reddit.com":     "www.reddit.com",
	"i.reddit.com":       "www.reddit.com",
	"np.reddit.com":      "www.reddit.com",
	"fr.reddit.com":      "www.reddit.com",
}

func cleanURL(ref string, refURL *url.URL) (string, *string, bool, bool) {
	// I'm not sure where these links are generated, but there are *a lot* of
	// them.
	if refURL.Host == "link.oreilly.com" {
		return "link.oreilly.com", nil, true, false
	}

	// Always remove protocol.
	refURL.Scheme = ""
	if p := strings.Index(ref, ":"); p > -1 && p < 7 {
		ref = ref[p+3:]
	}

	changed := false

	// Normalize some hosts.
	if a, ok := hostAlias[refURL.Host]; ok {
		changed = true
		refURL.Host = a
	}

	// Group based on URL.
	if strings.HasPrefix(refURL.Host, "www.google.") {
		// Group all "google.co.nz", "google.nl", etc. as "Google".
		return "Google", nil, true, true
	}
	if g, ok := groups[refURL.Host]; ok {
		return g, nil, true, true
	}

	// Special-fu for Feedly.
	if strings.HasPrefix(refURL.Host, "feedly.com") {
		// These URLs are all private, and we can't get any informatio from
		// them. Just list as "Feedly".
		//
		// https://feedly.com/i/collection/content/user/e5b84827-c85e-47db-81e6-15edd38e48f6/category/os-news
		// https://feedly.com/i/tag/user/34270c99-ef32-4b69-9e66-91f647b26247/tag/Test
		// https://feedly.com/i/category/programming
		if refURL.Path == "/i/latest" ||
			refURL.Path == "/i/my" ||
			refURL.Path == "/i/saved" ||
			strings.HasPrefix(refURL.Path, "/i/collection/") ||
			strings.HasPrefix(refURL.Path, "/i/tag/") ||
			strings.HasPrefix(refURL.Path, "/i/category/") {
			return "feedly.com", nil, true, false
		}

		// Subscriptions:
		// https://feedly.com/i/subscription/feed%2Fhttp%3A%2F%2Fafreshcup.com%2Ffeed%2F
		// https://feedly.com/i/subscription/feed%2Fhttp%3A%2F%2Fafreshcup.com%2Fhome%2Frss.xml
		// https://feedly.com/i/subscription/feed%2Fhttp%3A%2F%2Ffeeds.feedburner.com%2FCodrops
		// https://feedly.com/i/subscription/feed%2Fhttps%3A%2F%2Fnews.ycombinator.com%2Frss
		if strings.HasPrefix(refURL.Path, "/i/subscription/feed%2F") {
			p, err := url.PathUnescape(refURL.Path[23:])
			if err != nil {
				zlog.Error(err)
			} else {
				return p, nil, false, false
			}
		}

		// TODO: get feed from this too.
		// https://feedly.com/i/entry/+XHjch7MQtkDE3jVoUKNd7EXkxgLP+qd5d/qDPKdWEI=_16b1e5448ca:a8305:2a7e54a4
		// https://feedly.com/i/entry/1gOA8sgsyIN6Fa4oaXZX0qh2K2SOUMLVRi6qwkvVFZQ=_16a9fa31a3c:ac380:2a7e54a4
		// https://feedly.com/i/entry/5Td+U2A0pKfHcMqAZWYZgKWgpIItLeNiq7cfP1bAozw=_16b0df5c298:11e19b3:fe3711f1
	}

	// Useful: https://lobste.rs/s/tslw6k/why_i_m_still_using_jquery_2019
	// Not really: https://lobste.rs/newest/page/8, https://lobste.rs/page/7
	//             https://lobste.rs/search, https://lobste.rs/t/javascript
	if refURL.Host == "lobste.rs" && !strings.HasPrefix(refURL.Path, "/s/") {
		return "lobste.rs", nil, true, false
	}

	// Reddit
	// https://www.reddit.com/r/programming/top
	// https://www.reddit.com/r/programming/.compact
	// https://www.reddit.com/r/programming.compact
	// https://www.reddit.com/r/webdev/new
	if refURL.Host == "www.reddit.com" {
		switch {
		case strings.HasSuffix(refURL.Path, "/top") || strings.HasSuffix(refURL.Path, "/new"):
			refURL.Path = refURL.Path[:len(refURL.Path)-4]
			changed = true
		case strings.HasSuffix(refURL.Path, ".compact"):
			refURL.Path = refURL.Path[:len(refURL.Path)-8]
			changed = true
		}
	}

	// Clean query parameters.
	i := strings.Index(ref, "?")
	if i == -1 {
		// No parameters so no work.
		return strings.TrimLeft(refURL.String(), "/"), nil, changed, false
	}
	eq := ref[i+1:]
	ref = ref[:i]

	// Twitter's t.co links add this.
	if refURL.Host == "t.co" && eq == "amp=1" {
		return ref, nil, false, false
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
		return refURL.String()[2:], nil, changed || len(q) != start, false
	}
	eq = q.Encode()
	return refURL.String()[2:], &eq, changed || len(q) != start, false
}

// Defaults sets fields to default values, unless they're already set.
func (h *Hit) Defaults(ctx context.Context) {
	// TODO: not doing this as it's not set from memstore.
	// site := MustGetSite(ctx)
	// h.Site = site.ID

	if h.CreatedAt.IsZero() {
		h.CreatedAt = time.Now().UTC()
	}

	if h.Ref != "" && h.refURL != nil {
		if h.refURL.Scheme == "http" || h.refURL.Scheme == "https" {
			h.RefScheme = RefSchemeHTTP
		} else {
			h.RefScheme = RefSchemeOther
		}

		var store, generated bool
		r := h.Ref
		h.Ref, h.RefParams, store, generated = cleanURL(h.Ref, h.refURL)
		if store {
			h.RefOriginal = &r
		}

		if generated {
			h.RefScheme = RefSchemeGenerated
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

type Hits []Hit

// List all hits for a site.
func (h *Hits) List(ctx context.Context) error {
	return errors.Wrap(zdb.MustGet(ctx).SelectContext(ctx, h,
		`select * from hits where site=$1`, MustGetSite(ctx).ID),
		"Hits.List")
}

// Purge all paths matching the like pattern.
func (h *Hits) Purge(ctx context.Context, path string) error {
	_, err := zdb.MustGet(ctx).ExecContext(ctx,
		`delete from hits where site=$1 and lower(path) like lower($2)`,
		MustGetSite(ctx).ID, path)
	if err != nil {
		return errors.Wrap(err, "Hits.Purge")
	}

	_, err = zdb.MustGet(ctx).ExecContext(ctx,
		`delete from hit_stats where site=$1 and lower(path) like lower($2)`,
		MustGetSite(ctx).ID, path)
	return errors.Wrap(err, "Hits.Purge")
}

type HitStat struct {
	Day  string
	Days [][]int
}

type hs struct {
	Count     int     `db:"count"`
	Max       int     `db:"-"`
	Path      string  `db:"path"`
	RefScheme *string `db:"ref_scheme"`
	Stats     []HitStat
}

type HitStats []hs

func (h *HitStats) List(ctx context.Context, start, end time.Time, exclude []string) (int, int, bool, error) {
	db := zdb.MustGet(ctx)
	site := MustGetSite(ctx)

	limit := site.Settings.Limits.Page
	if limit == 0 {
		limit = 20
	}
	more := false
	if len(exclude) > 0 {
		// Get one page more so we can detect if there are more pages after
		// this.
		more = true
		limit++
	}

	query := `
		select
			path, count(path) as count
		from hits
		where
			site=? and
			created_at >= ? and
			created_at <= ?`
	args := []interface{}{site.ID, dayStart(start), dayEnd(end)}

	// Quite a bit faster to not check path.
	if len(exclude) > 0 {
		args = append(args, exclude)
		query += ` and path not in (?) `
	}

	query, args, err := sqlx.In(query+`
		group by path
		order by count desc
		limit ?`, append(args, limit)...)
	if err != nil {
		return 0, 0, false, errors.Wrap(err, "HitStats.List")
	}

	l := zlog.Module("HitStats.List")

	err = db.SelectContext(ctx, h, db.Rebind(query), args...)
	if err != nil {
		return 0, 0, false, errors.Wrap(err, "HitStats.List")
	}
	l = l.Since("select hits")

	if more {
		if len(*h) == limit {
			x := *h
			x = x[:len(x)-1]
			*h = x
		} else {
			more = false
		}
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
			day >= $2 and
			day <= $3
		order by day asc`,
		site.ID, start.Format("2006-01-02"), end.Format("2006-01-02"))
	if err != nil {
		return 0, 0, false, errors.Wrap(err, "HitStats.List")
	}
	l = l.Since("select hits_stats")

	// TODO: meh...
	hh := *h
	totalDisplay := 0
	for i := range hh {
		for _, s := range st {
			if s.Path == hh[i].Path {
				var x [][]int
				jsonutil.MustUnmarshal(s.Stats, &x)
				hh[i].Stats = append(hh[i].Stats, HitStat{Day: s.Day.Format("2006-01-02"), Days: x})

				// Get max.
				for j := range x {
					totalDisplay += x[j][1]
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

	l = l.Since("reorder data")

	// Get total.
	total := 0
	err = db.GetContext(ctx, &total, `
		select count(path)
		from hits
		where
			site=$1 and
			created_at >= $2 and
			created_at <= $3`,
		site.ID, dayStart(start), dayEnd(end))

	l = l.Since("get total")
	return total, totalDisplay, more, errors.Wrap(err, "HitStats.List")
}

// ListRefs lists all references for a path.
func (h *HitStats) ListRefs(ctx context.Context, path string, start, end time.Time, offset int) (bool, error) {
	site := MustGetSite(ctx)

	limit := site.Settings.Limits.Ref
	if limit == 0 {
		limit = 10
	}

	// TODO: using offset for pagination is not ideal:
	// data can change in the meanwhile, and it still gets the first N rows,
	// which is more expensive than it needs to be.
	// It's "good enough" for now, though.
	err := zdb.MustGet(ctx).SelectContext(ctx, h, `
		select
			ref as path,
			count(ref) as count,
			ref_scheme
		from hits
		where
			site=$1 and
			lower(path)=lower($2) and
			created_at >= $3 and
			created_at <= $4
		group by ref, ref_scheme
		order by count(*) desc, path desc
		limit $5 offset $6`,
		site.ID, path, dayStart(start), dayEnd(end), limit+1, offset)

	more := false
	if len(*h) > limit {
		more = true
		x := *h
		x = x[:len(x)-1]
		*h = x
	}

	return more, errors.Wrap(err, "RefStats.ListRefs")
}

// ListPaths lists all paths we have statistics for.
func (h *HitStats) ListPaths(ctx context.Context) ([]string, error) {
	var paths []string
	err := zdb.MustGet(ctx).SelectContext(ctx, &paths,
		`select path from hit_stats where site=$1 group by path`,
		MustGetSite(ctx).ID)
	return paths, errors.Wrap(err, "Hits.ListPaths")
}

// ListPathsLike lists all paths matching the like pattern.
func (h *HitStats) ListPathsLike(ctx context.Context, path string) error {
	err := zdb.MustGet(ctx).SelectContext(ctx, h, `
		select path, count(path) as count from hits
		where site=$1 and lower(path) like lower($2)
		group by path
		order by count desc
		`, MustGetSite(ctx).ID, path)
	return errors.Wrap(err, "Hits.ListPaths")
}

type BrowserStats []struct {
	Browser string
	Mobile  bool
	Count   int
}

// List all browser statistics for the given time period.
func (h *BrowserStats) List(ctx context.Context, start, end time.Time) (int, int, error) {
	err := zdb.MustGet(ctx).SelectContext(ctx, h, `
		select browser, sum(count) as count from browser_stats
		where site=$1 and day >= $2 and day <= $3
		group by browser 
		order by count desc
	`, MustGetSite(ctx).ID, start.Format("2006-01-02"), end.Format("2006-01-02"))
	if err != nil {
		return 0, 0, errors.Wrap(err, "BrowserStats.List browsers")
	}

	var total int
	for _, b := range *h {
		total += b.Count
	}

	// List number of mobile browsers.
	var m *int
	err = zdb.MustGet(ctx).GetContext(ctx, &m, `
		select sum(count) from browser_stats
		where site=$1 and day >= $2 and day <= $3 and mobile=true
	`, MustGetSite(ctx).ID, start.Format("2006-01-02"), end.Format("2006-01-02"))
	if err != nil {
		return 0, 0, errors.Wrap(err, "BrowserStats.List mobile")
	}

	mobile := 0
	if m != nil {
		mobile = *m
	}

	return total, mobile, nil
}

// ListBrowser lists all the versions for one browser.
func (h *BrowserStats) ListBrowser(ctx context.Context, browser string, start, end time.Time) (int, error) {
	err := zdb.MustGet(ctx).SelectContext(ctx, h, `
		select
			version as browser,
			sum(count) as count
		from browser_stats
		where site=$1 and day >= $2 and day <= $3 and lower(browser)=lower($4)
		group by browser, version
		order by count desc
	`, MustGetSite(ctx).ID, start.Format("2006-01-02"), end.Format("2006-01-02"), browser)
	if err != nil {
		return 0, errors.Wrap(err, "BrowserStats.ListBrowser")
	}

	var total int
	for _, b := range *h {
		total += b.Count
	}
	return total, nil
}

// ListSize lists all device sizes.
func (h *BrowserStats) ListSize(ctx context.Context, start, end time.Time) error {
	// TODO: just store better; all of this is ugly.
	// select split_part(size, ',', 1) || ',' || split_part(size, ',', 2) as browser,
	// order by cast(split_part(size, ',', 1) as int) asc
	err := zdb.MustGet(ctx).SelectContext(ctx, h, `
		select size as browser, count(size) as count
		from hits
		where
			site=$1 and
			created_at >= $2 and created_at <= $3
		group by size
	`, MustGetSite(ctx).ID, dayStart(start), dayEnd(end))
	if err != nil {
		return errors.Wrap(err, "BrowserStats.ListSize")
	}

	// hh := *h
	// for i := range hh {
	// 	s := strings.Split(hh[i].Browser, ", ")
	// 	hh[i].Browser = fmt.Sprintf("%s×%s", s[0], s[1])
	// }

	// sort.Slice(hh, func(i int, j int) bool {
	// 	p1, _ := strconv.ParseInt(hh[i].Browser[:strings.Index(hh[i].Browser, "×")], 10, 32)
	// 	p2, _ := strconv.ParseInt(hh[j].Browser[:strings.Index(hh[j].Browser, "×")], 10, 32)
	// 	return p1 < p2
	// })

	// TODO: group a bit; ideally I'd like to make a line chart in the future,
	// in which case this should no longer be needed.
	ns := BrowserStats{
		{"≤ 384×800", false, 0},
		{"≤ 1024×768", false, 0},
		{"≤ 1440×900", false, 0},
		{"≤ 1920×1080", false, 0},
		{"≤ 2560×1440", false, 0},
		{"> 2560×1440", false, 0},
		{"Unknown", false, 0},
	}
	hh := *h
	for i := range hh {
		x, _ := strconv.ParseInt(strings.Split(hh[i].Browser, ", ")[0], 10, 16)
		// TODO: apply scaling?
		switch {
		case x == 0:
			ns[6].Count += hh[i].Count
		case x <= 384:
			ns[0].Count += hh[i].Count
		case x <= 1024:
			ns[1].Count += hh[i].Count
		case x <= 1440:
			ns[2].Count += hh[i].Count
		case x <= 1920:
			ns[3].Count += hh[i].Count
		case x <= 2560:
			ns[4].Count += hh[i].Count
		default:
			ns[5].Count += hh[i].Count
		}
	}
	*h = ns
	//_ = ns
	return nil
}

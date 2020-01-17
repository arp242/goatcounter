// Copyright © 2019 Martin Tournoij <martin@arp242.net>
// This file is part of GoatCounter and published under the terms of the EUPL
// v1.2, which can be found in the LICENSE file or at http://eupl12.zgo.at

package goatcounter

import (
	"context"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/pkg/errors"
	"zgo.at/goatcounter/cfg"
	"zgo.at/utils/jsonutil"
	"zgo.at/utils/sqlutil"
	"zgo.at/zdb"
	"zgo.at/zlog"
	"zgo.at/zvalidate"
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
	Title       string            `db:"title" json:"t,omitempty"`
	Domain      string            `db:"domain" json:"d,omitempty"`
	Ref         string            `db:"ref" json:"r,omitempty"`
	RefParams   *string           `db:"ref_params" json:"-"`
	RefOriginal *string           `db:"ref_original" json:"-"`
	RefScheme   *string           `db:"ref_scheme" json:"-"`
	Browser     string            `db:"browser" json:"-"`
	Size        sqlutil.FloatList `db:"size" json:"s"`
	Location    string            `db:"location"`
	Bot         int               `db:"bot"`
	CreatedAt   time.Time         `db:"created_at" json:"-"`

	// Tracks referer of the /count request; this is not a statistic, just so we
	// can get an indication on which domains people are using GoatCounter, to
	// help track down abuse.
	CountRef string `db:"count_ref" json:"-"`

	// Parsed Ref
	RefURL *url.URL `db:"-"`
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
	"hnews.xyz":                          "Hacker News",
	"hackernewsmobile.com":               "Hacker News",
	// http://www.elegantreader.com/item/17358103
	// https://www.daemonology.net/hn-daily/2019-05.html

	"mail.google.com":       "Email",
	"com.google.android.gm": "Email",
	"mail.yahoo.com":        "Email",
	//  https://mailchi.mp

	"org.fox.ttrss":            "RSS",
	"www.inoreader.com":        "RSS",
	"com.innologica.inoreader": "RSS",
	"usepanda.com":             "RSS",
	"feedly.com":               "RSS",

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

	// Useful: https://lobste.rs/s/tslw6k/why_i_m_still_using_jquery_2019
	// Not really: https://lobste.rs/newest/page/8, https://lobste.rs/page/7
	//             https://lobste.rs/search, https://lobste.rs/t/javascript
	if refURL.Host == "lobste.rs" && !strings.HasPrefix(refURL.Path, "/s/") {
		return "lobste.rs", nil, true, false
	}
	if refURL.Host == "gambe.ro" && !strings.HasPrefix(refURL.Path, "/s/") {
		return "lobste.rs", nil, true, false
	}

	// Reddit
	// www.reddit.com/r/programming/top
	// www.reddit.com/r/programming/.compact
	// www.reddit.com/r/programming.compact
	// www.reddit.com/r/webdev/new
	// www.reddit.com/r/vim/search
	if refURL.Host == "www.reddit.com" {
		switch {
		case strings.HasSuffix(refURL.Path, "/top") || strings.HasSuffix(refURL.Path, "/new"):
			refURL.Path = refURL.Path[:len(refURL.Path)-4]
			changed = true
		case strings.HasSuffix(refURL.Path, "/search"):
			refURL.Path = refURL.Path[:len(refURL.Path)-7]
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
	if s := GetSite(ctx); s != nil && s.ID > 0 { // Not set from memstore.
		h.Site = s.ID
	}

	if h.CreatedAt.IsZero() {
		h.CreatedAt = time.Now().UTC()
	}

	if h.Ref != "" && h.RefURL != nil {
		if h.RefURL.Scheme == "http" || h.RefURL.Scheme == "https" {
			h.RefScheme = RefSchemeHTTP
		} else {
			h.RefScheme = RefSchemeOther
		}

		var store, generated bool
		r := h.Ref
		h.Ref, h.RefParams, store, generated = cleanURL(h.Ref, h.RefURL)
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
	v := zvalidate.New()

	v.Required("site", h.Site)
	v.Required("path", h.Path)

	return v.ErrorOrNil()
}

type Hits []Hit

// List all hits for a site.
func (h *Hits) List(ctx context.Context) error {
	return errors.Wrap(zdb.MustGet(ctx).SelectContext(ctx, h,
		`select * from hits where site=$1 and bot=0`, MustGetSite(ctx).ID),
		"Hits.List")
}

// Purge all paths matching the like pattern.
func (h *Hits) Purge(ctx context.Context, path string) error {
	return zdb.TX(ctx, func(ctx context.Context, tx zdb.DB) error {
		site := MustGetSite(ctx).ID

		_, err := tx.ExecContext(ctx,
			`delete from hits where site=$1 and lower(path) like lower($2)`,
			site, path)
		if err != nil {
			return errors.Wrap(err, "Hits.Purge")
		}

		_, err = tx.ExecContext(ctx,
			`delete from hit_stats where site=$1 and lower(path) like lower($2)`,
			site, path)
		if err != nil {
			return errors.Wrap(err, "Hits.Purge")
		}

		return nil
	})
}

type Stat struct {
	Day  string
	Days []int
}

type HitStat struct {
	Count     int     `db:"count"`
	Max       int     `db:"-"`
	Path      string  `db:"path"`
	Title     string  `db:"title"`
	RefScheme *string `db:"ref_scheme"`
	Stats     []Stat
}

type HitStats []HitStat

var allDays = []int{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}

func (h *HitStats) List(ctx context.Context, start, end time.Time, filter string, exclude []string) (int, int, bool, error) {
	db := zdb.MustGet(ctx)
	site := MustGetSite(ctx)
	l := zlog.Module("HitStats.List")

	limit := site.Settings.Limits.Page
	if limit == 0 {
		limit = 20
	}
	more := false
	if len(exclude) > 0 || filter != "" {
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
			bot=0 and
			created_at >= ? and
			created_at <= ? `
	args := []interface{}{site.ID, dayStart(start), dayEnd(end)}

	if filter != "" {
		filter = "%" + strings.ToLower(filter) + "%"
		query += ` and (lower(path) like ? or lower(title) like ?) `
		args = append(args, filter, filter)
	}

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

	err = db.SelectContext(ctx, h, db.Rebind(query), args...)
	if err != nil {
		return 0, 0, false, errors.Wrap(err, "HitStats.List")
	}
	l = l.Since("select hits")

	// TODO: not always correct
	// TODO: breaks after 2 pages?
	// More 3 3 []
	// &[{2380 0 /tmux.html  <nil> []} {230 0 /the-art-of-unix-programming  <nil> []} {95 0 /taoup.html  <nil> []}]
	//    13:01:07 backend: INFO:  {browsers.List="1ms" locStat.List="1ms" pages.List="85ms" site=1 sizeStat.ListSizes="26ms" zhttp.Template="2ms"}
	// 13:01:07  200 GET   118ms  arp242.goatcounter.localhost:8081/?filter=x
	// More 3 3 [/tmux.html /the-art-of-unix-programming ]
	// &[{95 0 /taoup.html  <nil> []} {70 0 /read-stdin.html  <nil> []} {57 0 /anti-vax.html  <nil> []}]
	// 13:01:11  200 GET    78ms  arp242.goatcounter.localhost:8081/pages?period-start=2020-01-10&period-end=2020-01-17&filter=x&exclude=%2ftmux.html,%2fthe-art-of-unix-programming,
	// More 0 3 [/tmux.html /the-art-of-unix-programming /taoup.html /read-stdin.html]
	// &[]
	// 13:01:12  200 GET    88ms  arp242.goatcounter.localhost:8081/pages?period-start=2020-01-10&period-end=2020-01-17&filter=false&exclude=%2Ftmux.html%2C%2Fthe-art-of-unix-programming%2C%2Ftaoup.html%2C%2Fread-stdin.html
	if more {
		fmt.Println("More", len(*h), limit, exclude)
		fmt.Println(h)
		if len(*h) == limit {
			x := *h
			x = x[:len(x)-1]
			*h = x
		} else {
			more = false
		}
	}

	// Add stats and title
	type stats struct {
		Path  string    `db:"path"`
		Title string    `db:"title"`
		Day   time.Time `db:"day"`
		Stats []byte    `db:"stats"`
	}
	query = `
		select path, title, day, stats
		from hit_stats
		where
			site=$1 and
			day >= $2 and
			day <= $3 `
	args = []interface{}{site.ID, start.Format("2006-01-02"), end.Format("2006-01-02")}
	if filter != "" {
		query += ` and (lower(path) like $4 or lower(title) like $4) `
		args = append(args, filter)
	}
	query += ` order by day asc`

	var st []stats
	err = db.SelectContext(ctx, &st, query, args...)
	if err != nil {
		return 0, 0, false, errors.Wrap(err, "HitStats.List")
	}
	l = l.Since("select hits_stats")

	// Get max amount and totals.
	hh := *h
	totalDisplay := 0
	for i := range hh {
		for _, s := range st {
			if s.Path == hh[i].Path {
				var x []int
				jsonutil.MustUnmarshal(s.Stats, &x)
				hh[i].Title = s.Title
				hh[i].Stats = append(hh[i].Stats, Stat{Day: s.Day.Format("2006-01-02"), Days: x})

				// Get max.
				for j := range x {
					totalDisplay += x[j]
					if x[j] > hh[i].Max {
						hh[i].Max = x[j]
					}
				}
			}
		}

		if hh[i].Max < 10 {
			hh[i].Max = 10
		}
	}
	l = l.Since("reorder data")

	// Fill in blank days.
	endFmt := end.Format("2006-01-02")
	for i := range hh {
		var (
			day     = start.Add(-24 * time.Hour)
			newStat []Stat
			j       int
		)
		if day.Before(site.CreatedAt) {
			day = site.CreatedAt.Add(-24 * time.Hour)
		}

		for {
			day = day.Add(24 * time.Hour)
			dayFmt := day.Format("2006-01-02")

			if len(hh[i].Stats)-1 >= j && dayFmt == hh[i].Stats[j].Day {
				newStat = append(newStat, hh[i].Stats[j])
				j++
			} else {
				newStat = append(newStat, Stat{Day: dayFmt, Days: allDays})
			}
			if dayFmt == endFmt {
				break
			}
		}
		hh[i].Stats = newStat
	}
	l = l.Since("fill blanks")

	// Get total.
	query = `
		select count(path)
		from hits
		where
			site=$1 and
			bot=0 and
			created_at >= $2 and
			created_at <= $3 `
	args = []interface{}{site.ID, dayStart(start), dayEnd(end)}
	if filter != "" {
		query += ` and (lower(path) like $4 or lower(title) like $4) `
		args = append(args, filter)
	}

	total := 0
	err = db.GetContext(ctx, &total, query, args...)

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
			bot=0 and
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
		where site=$1 and bot=0 and lower(path) like lower($2)
		group by path
		order by count desc
		`, MustGetSite(ctx).ID, path)
	return errors.Wrap(err, "Hits.ListPaths")
}

type Stats []struct {
	Name   string
	Mobile bool
	Count  int
}

// List all browser statistics for the given time period.
func (h *Stats) ListBrowsers(ctx context.Context, start, end time.Time) (int, int, error) {
	err := zdb.MustGet(ctx).SelectContext(ctx, h, `
		select browser as name, sum(count) as count from browser_stats
		where site=$1 and day >= $2 and day <= $3
		group by browser 
		order by count desc
	`, MustGetSite(ctx).ID, start.Format("2006-01-02"), end.Format("2006-01-02"))
	if err != nil {
		return 0, 0, errors.Wrap(err, "Stats.ListBrowsers browsers")
	}

	var total int
	for _, b := range *h {
		total += b.Count
	}

	// List number of mobile browsers.
	// TODO: inaccurate and not shown in UI at the moment.
	//var m *int
	//err = zdb.MustGet(ctx).GetContext(ctx, &m, `
	//	select sum(count) from browser_stats
	//	where site=$1 and day >= $2 and day <= $3 and mobile=true
	//`, MustGetSite(ctx).ID, start.Format("2006-01-02"), end.Format("2006-01-02"))
	//if err != nil {
	//	return 0, 0, errors.Wrap(err, "Stats.ListBrowsers mobile")
	//}

	mobile := 0
	//if m != nil {
	//	mobile = *m
	//}

	return total, mobile, nil
}

// ListBrowser lists all the versions for one browser.
func (h *Stats) ListBrowser(ctx context.Context, browser string, start, end time.Time) (int, error) {
	err := zdb.MustGet(ctx).SelectContext(ctx, h, `
		select
			browser || ' ' || version as name,
			sum(count) as count
		from browser_stats
		where site=$1 and day >= $2 and day <= $3 and lower(browser)=lower($4)
		group by browser, version
		order by count desc
	`, MustGetSite(ctx).ID, start.Format("2006-01-02"), end.Format("2006-01-02"), browser)
	if err != nil {
		return 0, errors.Wrap(err, "Stats.ListBrowser")
	}

	var total int
	for _, b := range *h {
		total += b.Count
	}
	return total, nil
}

const (
	sizePhones      = "Phones"
	sizeLargePhones = "Large phones, small tablets"
	sizeTablets     = "Tablets and small laptops"
	sizeDesktop     = "Computer monitors"
	sizeDesktopHD   = "Computer monitors larger than HD"
	sizeUnknown     = "(unknown)"
)

// ListSizes lists all device sizes.
func (h *Stats) ListSizes(ctx context.Context, start, end time.Time) (int, error) {
	// TODO: just store better; all of this is ugly.
	// select split_part(size, ',', 1) || ',' || split_part(size, ',', 2) as browser,
	// order by cast(split_part(size, ',', 1) as int) asc
	err := zdb.MustGet(ctx).SelectContext(ctx, h, `
		select size as name, count(size) as count
		from hits
		where
			site=$1 and
			bot=0 and
			created_at >= $2 and created_at <= $3
		group by size
	`, MustGetSite(ctx).ID, dayStart(start), dayEnd(end))
	if err != nil {
		return 0, errors.Wrap(err, "Stats.ListSize")
	}

	// hh := *h
	// for i := range hh {
	// 	s := strings.Split(hh[i].Name, ", ")
	// 	hh[i].Name = fmt.Sprintf("%s×%s", s[0], s[1])
	// }
	// sort.Slice(hh, func(i int, j int) bool {
	// 	p1, _ := strconv.ParseInt(hh[i].Name[:strings.Index(hh[i].Name, "×")], 10, 32)
	// 	p2, _ := strconv.ParseInt(hh[j].Name[:strings.Index(hh[j].Name, "×")], 10, 32)
	// 	return p1 < p2
	// })

	// TODO: group a bit; ideally I'd like to make a line chart in the future,
	// in which case this should no longer be needed.
	ns := Stats{
		{sizePhones, false, 0},
		{sizeLargePhones, false, 0},
		{sizeTablets, false, 0},
		{sizeDesktop, false, 0},
		{sizeDesktopHD, false, 0},
		{sizeUnknown, false, 0},
	}

	hh := *h
	var count int
	for i := range hh {
		count += hh[i].Count

		x, _ := strconv.ParseInt(strings.Split(hh[i].Name, ", ")[0], 10, 16)
		// TODO: apply scaling?
		switch {
		case x == 0:
			ns[5].Count += hh[i].Count
		case x <= 384:
			ns[0].Count += hh[i].Count
		case x <= 1024:
			ns[1].Count += hh[i].Count
		case x <= 1440:
			ns[2].Count += hh[i].Count
		case x <= 1920:
			ns[3].Count += hh[i].Count
		default:
			ns[4].Count += hh[i].Count
		}
	}
	*h = ns

	return count, nil
}

// ListSize lists all sizes for one grouping.
func (h *Stats) ListSize(ctx context.Context, name string, start, end time.Time) (int, error) {
	var where string
	switch name {
	case sizePhones:
		where = "size != '' and cast(split_part(size, ',', 1) as int) <= 384"
	case sizeLargePhones:
		where = "size != '' and cast(split_part(size, ',', 1) as int) <= 1024 and cast(split_part(size, ',', 1) as int) > 384"
	case sizeTablets:
		where = "size != '' and cast(split_part(size, ',', 1) as int) <= 1440 and cast(split_part(size, ',', 1) as int) > 1024"
	case sizeDesktop:
		where = "size != '' and cast(split_part(size, ',', 1) as int) <= 1920 and cast(split_part(size, ',', 1) as int) > 1440"
	case sizeDesktopHD:
		where = "size != '' and cast(split_part(size, ',', 1) as int) > 1920"
	case sizeUnknown:
		where = "size = ''"
	default:
		return 0, fmt.Errorf("Stats.ListSizes: invalid value for name: %#v", name)
	}

	if !cfg.PgSQL {
		where = strings.Replace(where,
			"split_part(size, ',', 1)",
			"substr(size, 0, instr(size, ','))",
			-1)
	}

	err := zdb.MustGet(ctx).SelectContext(ctx, h, fmt.Sprintf(`
		select size as name, count(size) as count
		from hits
		where
			site=$1 and
			bot=0 and
			created_at >= $2 and created_at <= $3 and
			%s
		group by size
	`, where), MustGetSite(ctx).ID, dayStart(start), dayEnd(end))
	if err != nil {
		return 0, errors.Wrap(err, "Stats.ListSize")
	}

	grouped := make(map[string]int)
	hh := *h
	for i := range hh {
		// TODO: apply scaling?
		grouped[fmt.Sprintf("↔ %spx", strings.Split(hh[i].Name, ", ")[0])] += hh[i].Count
	}

	ns := Stats{}
	total := 0
	for size, count := range grouped {
		total += count
		ns = append(ns, struct {
			Name   string
			Mobile bool
			Count  int
		}{size, false, count})
	}
	sort.Slice(ns, func(i int, j int) bool { return ns[i].Count > ns[j].Count })
	*h = ns

	return total, nil
}

// List all location statistics for the given time period.
func (h *Stats) ListLocations(ctx context.Context, start, end time.Time) (int, error) {
	err := zdb.MustGet(ctx).SelectContext(ctx, h, `
		select
			iso_3166_1.name as name,
			sum(count) as count
		from location_stats
		join iso_3166_1 on iso_3166_1.alpha2=location
		where site=$1 and day >= $2 and day <= $3
		group by location, iso_3166_1.name
		order by count desc
	`, MustGetSite(ctx).ID, start.Format("2006-01-02"), end.Format("2006-01-02"))
	if err != nil {
		return 0, errors.Wrap(err, "Stats.ListLocations")
	}

	var total int
	for _, b := range *h {
		total += b.Count
	}

	return total, nil
}

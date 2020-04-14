// Copyright © 2019 Martin Tournoij <martin@arp242.net>
// This file is part of GoatCounter and published under the terms of the EUPL
// v1.2, which can be found in the LICENSE file or at http://eupl12.zgo.at

package goatcounter

import (
	"bytes"
	"context"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/jmoiron/sqlx"
	"zgo.at/goatcounter/errors"
	"zgo.at/utils/jsonutil"
	"zgo.at/utils/mathutil"
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
	ID      int64  `db:"id" json:"-"`
	Site    int64  `db:"site" json:"-"`
	Session *int64 `db:"session" json:"-"`

	Path  string            `db:"path" json:"p,omitempty"`
	Title string            `db:"title" json:"t,omitempty"`
	Ref   string            `db:"ref" json:"r,omitempty"`
	Event sqlutil.Bool      `db:"event" json:"e,omitempty"`
	Size  sqlutil.FloatList `db:"size" json:"s,omitempty"`

	RefParams      *string      `db:"ref_params" json:"-"`
	RefOriginal    *string      `db:"ref_original" json:"-"`
	RefScheme      *string      `db:"ref_scheme" json:"-"`
	Browser        string       `db:"browser" json:"-"`
	Location       string       `db:"location" json:"-"`
	StartedSession sqlutil.Bool `db:"started_session" json:"-"`
	Bot            int          `db:"bot" json:"-"`
	CreatedAt      time.Time    `db:"created_at" json:"-"`

	RefURL      *url.URL `db:"-" json:"-"`   // Parsed Ref
	UsageDomain string   `db:"-" json:"-"`   // Track referrer for usage.
	Random      string   `db:"-" json:"rnd"` // Browser cache buster, as they don't always listen to Cache-Control
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

func cleanPath(path string) string {
	// No query parameters.
	if !strings.Contains(path, "?") {
		return path
	}

	u, err := url.Parse(path)
	if err != nil {
		return path
	}

	q := u.Query()

	// Magic Facebook tracking parameter. As far as I can find it's not public
	// what this even does exactly, so just remove it to prevent pages from
	// being show more than once.
	q.Del("fbclid")

	u.RawQuery = q.Encode()
	return u.String()
}

func (h Hit) String() string {
	b := new(bytes.Buffer)
	t := tabwriter.NewWriter(b, 8, 8, 2, ' ', 0)
	fmt.Fprintf(t, "ID\t%d\n", h.ID)
	fmt.Fprintf(t, "Site\t%d\n", h.Site)
	if h.Session == nil {
		fmt.Fprintf(t, "Session\t<nil>\n")
	} else {
		fmt.Fprintf(t, "Session\t%d\n", *h.Session)
	}
	fmt.Fprintf(t, "Path\t%q\n", h.Path)
	fmt.Fprintf(t, "Title\t%q\n", h.Title)
	fmt.Fprintf(t, "Ref\t%q\n", h.Ref)
	fmt.Fprintf(t, "Event\t%t\n", h.Event)
	if h.RefParams == nil {
		fmt.Fprintf(t, "RefParams\t<nil>\n")
	} else {
		fmt.Fprintf(t, "RefParams\t%q\n", *h.RefParams)
	}
	if h.RefOriginal == nil {
		fmt.Fprintf(t, "RefOriginal\t<nil>\n")
	} else {
		fmt.Fprintf(t, "RefOriginal\t%q\n", *h.RefOriginal)
	}
	if h.RefScheme == nil {
		fmt.Fprintf(t, "RefScheme\t<nil>\n")
	} else {
		fmt.Fprintf(t, "RefScheme\t%q\n", *h.RefScheme)
	}
	fmt.Fprintf(t, "Browser\t%q\n", h.Browser)
	fmt.Fprintf(t, "Size\t%q\n", h.Size)
	fmt.Fprintf(t, "Location\t%q\n", h.Location)
	fmt.Fprintf(t, "Bot\t%d\n", h.Bot)
	fmt.Fprintf(t, "CreatedAt\t%s\n", h.CreatedAt)
	t.Flush()
	return b.String()
}

// Defaults sets fields to default values, unless they're already set.
func (h *Hit) Defaults(ctx context.Context) {
	if s := GetSite(ctx); s != nil && s.ID > 0 { // Not set from memstore.
		h.Site = s.ID
	}

	if h.CreatedAt.IsZero() {
		h.CreatedAt = Now()
	}

	if !h.Event {
		h.Path = cleanPath(h.Path)
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
	if !h.Event {
		h.Path = "/" + strings.Trim(h.Path, "/")
	}
}

// Validate the object.
func (h *Hit) Validate(ctx context.Context) error {
	v := zvalidate.New()

	v.Required("site", h.Site)
	v.Required("session", h.Session)
	v.Required("path", h.Path)
	v.UTF8("path", h.Path)
	v.UTF8("title", h.Title)
	v.UTF8("ref", h.Ref)
	v.UTF8("browser", h.Browser)
	v.UTF8("usage_domain", h.UsageDomain)

	v.Len("path", h.Path, 0, 2048)
	v.Len("title", h.Title, 0, 1024)
	v.Len("ref", h.Ref, 0, 2048)
	v.Len("browser", h.Browser, 0, 512)

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

		// Delete all other stats as well if there's nothing left: not much use
		// for it.
		var check Hits
		err = check.List(ctx)
		if err == nil && len(check) == 0 {
			for _, t := range statTables {
				_, err := tx.ExecContext(ctx, `delete from `+t+` where site=$1`, site)
				if err != nil {
					zlog.Errorf("Hits.Purge: delete %s: %s", t, err)
				}
			}
		}

		return nil
	})
}

type Stat struct {
	Day   string
	Days  []int
	Daily int
}

type HitStat struct {
	Count     int          `db:"count"`
	Max       int          `db:"-"`
	DailyMax  int          `db:"-"`
	Path      string       `db:"path"`
	Event     sqlutil.Bool `db:"event"`
	Title     string       `db:"title"`
	RefScheme *string      `db:"ref_scheme"`
	Stats     []Stat
}

type HitStats []HitStat

var allDays = []int{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}

func (h *HitStats) List(ctx context.Context, start, end time.Time, filter string, exclude []string) (int, int, bool, error) {
	db := zdb.MustGet(ctx)
	site := MustGetSite(ctx)
	l := zlog.Module("HitStats.List")

	// Get one page more so we can detect if there are more pages after this.
	limit := int(mathutil.NonZero(int64(site.Settings.Limits.Page), 10)) + 1

	// Select hits.
	var st []struct {
		Path  string       `db:"path"`
		Title string       `db:"title"`
		Event sqlutil.Bool `db:"event"`
		Day   time.Time    `db:"day"`
		Stats []byte       `db:"stats"`
	}
	var more bool
	{
		query := `
			select path from hits
			where
				site=? and
				bot=0 and
				created_at >= ? and
				created_at <= ? `
		args := []interface{}{site.ID, start, end}

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
			order by count(path) desc, path desc
			limit ?`, append(args, limit)...)
		if err != nil {
			return 0, 0, false, errors.Wrap(err, "HitStats.List")
		}
		err = db.SelectContext(ctx, h, db.Rebind(query), args...)
		if err != nil {
			return 0, 0, false, errors.Wrap(err, "HitStats.List")
		}
		l = l.Since("select hits")

		// Check if there are more entries.
		if len(*h) == limit {
			x := *h
			x = x[:len(x)-1]
			*h = x
			more = true
		}
	}

	// Add stats and title.
	{
		query := `
			select path, event, title, day, stats
			from hit_stats
			where
				site=$1 and
				day >= $2 and
				day <= $3 `
		args := []interface{}{site.ID, start.Format("2006-01-02"), end.Format("2006-01-02")}
		if filter != "" {
			query += ` and (lower(path) like $4 or lower(title) like $4) `
			args = append(args, filter)
		}
		query += ` order by day asc`
		err := db.SelectContext(ctx, &st, query, args...)
		if err != nil {
			return 0, 0, false, errors.Wrap(err, "HitStats.List")
		}
		l = l.Since("select hits_stats")
	}

	hh := *h

	// Add the hit_stats.
	{
		for i := range hh {
			for _, s := range st {
				if s.Path == hh[i].Path {
					var x []int
					jsonutil.MustUnmarshal(s.Stats, &x)
					hh[i].Title = s.Title
					hh[i].Event = s.Event
					hh[i].Stats = append(hh[i].Stats, Stat{Day: s.Day.Format("2006-01-02"), Days: x})
				}
			}
		}
		l = l.Since("add hit_stats")
	}

	// Fill in blank days.
	{
		endFmt := end.Format("2006-01-02")
		for i := range hh {
			var (
				day     = start.Add(-24 * time.Hour)
				newStat []Stat
				j       int
			)

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
	}

	// Apply TZ offset.
	{
		offset := site.Settings.Timezone.Offset()
		if offset%60 != 0 {
			offset += 30
		}
		offset /= 60

		for i := range hh {
			hh[i].Stats = applyOffset(offset, hh[i].Stats)
		}
		l = l.Since("tz")
	}

	// Add total and max.
	var totalDisplay int
	{
		for i := range hh {
			for j := range hh[i].Stats {
				for k := range hh[i].Stats[j].Days {
					hh[i].Stats[j].Daily += hh[i].Stats[j].Days[k]

					if hh[i].Stats[j].Days[k] > hh[i].Max {
						hh[i].Max = hh[i].Stats[j].Days[k]
					}
				}
				if hh[i].Stats[j].Daily > hh[i].DailyMax {
					hh[i].DailyMax = hh[i].Stats[j].Daily
				}
				hh[i].Count += hh[i].Stats[j].Daily
			}

			totalDisplay += hh[i].Count
			if hh[i].Max < 10 {
				hh[i].Max = 10
			}
			if hh[i].DailyMax < 10 {
				hh[i].DailyMax = 10
			}
		}

		// We sort in SQL, but this is not always 100% correct after applying
		// the TZ offset, so order here as well.
		// TODO: this is still not 100% correct, as the "first 10" after
		// applying the TZ offset may be different than the first 10 being
		// fetched in the SQL query. There is no easy fix for that in the
		// current design. I considered storing everything in the DB as the
		// configured TZ, but that would make changing the TZ expensive, I'm not
		// 100% sure yet what a good solution here is. For now, this is "good
		// enough".
		sort.Slice(hh, func(i, j int) bool { return hh[i].Count > hh[j].Count })
		l = l.Since("add totals")
	}

	// Get total number of hits in the selected time range
	// TODO: not 100% correct as it doesn't correct for TZ.
	var total int
	{
		query := `
		    select count(path)
		    from hits
		    where
			    site=$1 and
			    bot=0 and
			    created_at >= $2 and
			    created_at <= $3 `
		args := []interface{}{site.ID, start, end}
		if filter != "" {
			query += ` and (lower(path) like $4 or lower(title) like $4) `
			args = append(args, filter)
		}
		err := db.GetContext(ctx, &total, query, args...)
		if err != nil {
			return 0, 0, false, errors.Wrap(err, "HitStats.List")
		}

		l = l.Since("get total")
	}

	return total, totalDisplay, more, nil
}

// The database stores everything in UTC, so we need to apply
// the offset for HitStats.List()
//
// Let's say we have two days with an offset of UTC+2, this means we
// need to transform this:
//
//    2019-12-05 → [0,0,0,0,0,0,0,0,0,0,0,4,7,0,0,0,0,0,0,0,0,0,1,0]
//    2019-12-06 → [0,0,0,0,0,0,0,0,0,0,0,4,7,0,0,0,0,0,0,0,0,0,1,0]
//    2019-12-07 → [0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0]
//
// To:
//
//    2019-12-05 → [0,0,0,0,0,0,0,0,0,0,0,0,0,4,7,0,0,0,0,0,0,0,0,0]
//    2019-12-06 → [1,0,0,0,0,0,0,0,0,0,0,0,0,4,7,0,0,0,0,0,0,0,0,0]
//    2019-12-07 → [1,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0]
//
// And skip the first 2 hours of the first day.
//
// Or, for UTC-2:
//
//    2019-12-04 → [0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0]
//    2019-12-05 → [0,0,0,0,0,0,0,0,0,4,7,0,0,0,0,0,0,0,0,0,1,0,0,0]
//    2019-12-06 → [0,0,0,0,0,0,0,0,0,4,7,0,0,0,0,0,0,0,0,0,1,0,0,0]
//
// And skip the last 2 hours of the last day.
//
// Offsets that are not whole hours (e.g. 6:30) are treated like 7:00. I don't
// know how to do that otherwise.
func applyOffset(offset int, stats []Stat) []Stat {
	switch {
	case offset > 0:
		popped := make([]int, offset)
		for i := range stats {
			stats[i].Days = append(popped, stats[i].Days...)
			o := len(stats[i].Days) - offset
			popped = stats[i].Days[o:]
			stats[i].Days = stats[i].Days[:o]
		}
		stats = stats[1:] // Overselect a day to get the stats for it, remove it.

	case offset < 0:
		offset = -offset
		popped := make([]int, offset)
		for i := len(stats) - 1; i >= 0; i-- {
			stats[i].Days = append(stats[i].Days, popped...)
			popped = stats[i].Days[:offset]
			stats[i].Days = stats[i].Days[offset:]
		}
		stats = stats[:len(stats)-1] // Overselect a day to get the stats for it, remove it.
	}

	return stats
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
		order by count(ref) desc, path desc
		limit $5 offset $6`,
		site.ID, path, start, end, limit+1, offset)

	var more bool
	if len(*h) > limit {
		more = true
		hh := *h
		hh = hh[:len(hh)-1]
		*h = hh
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
	Name  string
	Count int
}

// ByRef lists all paths by reference.
func (h *Stats) ByRef(ctx context.Context, start, end time.Time, ref string) (int, error) {
	err := zdb.MustGet(ctx).SelectContext(ctx, h, `
		select path as name, count(path) as count
		from hits where
			site=$1 and
			bot=0 and
			created_at >= $2 and
			created_at <= $3 and
			ref = $4
		group by path
		order by count desc
	`, MustGetSite(ctx).ID, start, end, ref)

	var total int
	for _, b := range *h {
		total += b.Count
	}
	return total, errors.Wrap(err, "HitStats.ByRef")
}

// List all ref statistics for the given time period, excluding referrals from
// the configured LinkDomain.
//
// The returned count is the count without LinkDomain, and is different from the
// total number of hits.
func (h *Stats) ListRefs(ctx context.Context, start, end time.Time, limit, offset int) (int, bool, error) {
	site := MustGetSite(ctx)

	where := ` where site=? and day>=? and day<=?`
	args := []interface{}{site.ID, start.Format("2006-01-02"), end.Format("2006-01-02")}
	if site.LinkDomain != "" {
		where += " and ref not like ? "
		args = append(args, site.LinkDomain+"%")
	}

	db := zdb.MustGet(ctx)
	err := db.SelectContext(ctx, h, db.Rebind(`
		select ref as name, sum(count) as count from ref_stats`+
		where+`
		group by ref
		order by count desc
		limit ? offset ?`), append(args, limit+1, offset)...)
	if err != nil {
		return 0, false, errors.Wrap(err, "Stats.ListRefs")
	}

	var total int
	err = db.GetContext(ctx, &total,
		db.Rebind(`select coalesce(sum(count), 0) from ref_stats`+where),
		args...)
	if err != nil {
		return 0, false, errors.Wrap(err, "Stats.ListRefs: total")
	}

	var more bool
	if len(*h) > limit {
		more = true
		hh := *h
		hh = hh[:len(hh)-1]
		*h = hh
	}

	return total, more, nil
}

// List all browser statistics for the given time period.
func (h *Stats) ListBrowsers(ctx context.Context, start, end time.Time) (int, error) {
	err := zdb.MustGet(ctx).SelectContext(ctx, h, `
		select browser as name, sum(count) as count from browser_stats
		where site=$1 and day >= $2 and day <= $3
		group by browser
		order by count desc
	`, MustGetSite(ctx).ID, start.Format("2006-01-02"), end.Format("2006-01-02"))
	if err != nil {
		return 0, errors.Wrap(err, "Stats.ListBrowsers browsers")
	}

	var total int
	for _, b := range *h {
		total += b.Count
	}

	return total, nil
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
	err := zdb.MustGet(ctx).SelectContext(ctx, h, `
		select width as name, sum(count) as count
		from size_stats
		where site=$1 and day >= $2 and day <= $3
		group by width
		order by count desc
	`, MustGetSite(ctx).ID, start.Format("2006-01-02"), end.Format("2006-01-02"))
	if err != nil {
		return 0, errors.Wrap(err, "Stats.ListSize")
	}

	// Group a bit more user-friendly.
	// TODO: ideally I'd like to make a line chart in the future, in which case
	// this should no longer be needed.
	ns := Stats{
		{sizePhones, 0},
		{sizeLargePhones, 0},
		{sizeTablets, 0},
		{sizeDesktop, 0},
		{sizeDesktopHD, 0},
		{sizeUnknown, 0},
	}

	hh := *h
	var count int
	for i := range hh {
		count += hh[i].Count

		x, _ := strconv.ParseInt(hh[i].Name, 10, 16)
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
		where = "width != 0 and width <= 384"
	case sizeLargePhones:
		where = "width != 0 and width <= 1024 and width > 384"
	case sizeTablets:
		where = "width != 0 and width <= 1440 and width > 1024"
	case sizeDesktop:
		where = "width != 0 and width <= 1920 and width > 1440"
	case sizeDesktopHD:
		where = "width != 0 and width > 1920"
	case sizeUnknown:
		where = "width = 0"
	default:
		return 0, fmt.Errorf("Stats.ListSizes: invalid value for name: %#v", name)
	}

	err := zdb.MustGet(ctx).SelectContext(ctx, h, fmt.Sprintf(`
		select width as name, sum(count) as count
		from size_stats
		where
			site=$1 and day >= $2 and day <= $3 and
			%s
		group by width
	`, where), MustGetSite(ctx).ID, start, end)
	if err != nil {
		return 0, errors.Wrap(err, "Stats.ListSize")
	}

	grouped := make(map[string]int)
	hh := *h
	for i := range hh {
		grouped[fmt.Sprintf("↔ %spx", hh[i].Name)] += hh[i].Count
	}

	ns := Stats{}
	total := 0
	for width, count := range grouped {
		total += count
		ns = append(ns, struct {
			Name  string
			Count int
		}{width, count})
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

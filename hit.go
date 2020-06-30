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

	"zgo.at/errors"
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
	RefSchemeCampaign  = ptr("c")
)

type Hit struct {
	ID      int64  `db:"id" json:"-"`
	Site    int64  `db:"site" json:"-"`
	Session *int64 `db:"session" json:"-"`

	Path  string     `db:"path" json:"p,omitempty"`
	Title string     `db:"title" json:"t,omitempty"`
	Ref   string     `db:"ref" json:"r,omitempty"`
	Event zdb.Bool   `db:"event" json:"e,omitempty"`
	Size  zdb.Floats `db:"size" json:"s,omitempty"`
	Query string     `db:"-" json:"q,omitempty"`
	Bot   int        `db:"bot" json:"b,omitempty"`

	RefScheme  *string   `db:"ref_scheme" json:"-"`
	Browser    string    `db:"browser" json:"-"`
	Location   string    `db:"location" json:"-"`
	FirstVisit zdb.Bool  `db:"first_visit" json:"-"`
	CreatedAt  time.Time `db:"created_at" json:"-"`

	RefURL *url.URL `db:"-" json:"-"`   // Parsed Ref
	Random string   `db:"-" json:"rnd"` // Browser cache buster, as they don't always listen to Cache-Control
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

func cleanRefURL(ref string, refURL *url.URL) (string, bool) {
	// I'm not sure where these links are generated, but there are *a lot* of
	// them.
	if refURL.Host == "link.oreilly.com" {
		return "link.oreilly.com", false
	}

	// Always remove protocol.
	refURL.Scheme = ""
	if p := strings.Index(ref, ":"); p > -1 && p < 7 {
		ref = ref[p+3:]
	}

	// Normalize some hosts.
	if a, ok := hostAlias[refURL.Host]; ok {
		refURL.Host = a
	}

	// Group based on URL.
	if strings.HasPrefix(refURL.Host, "www.google.") {
		// Group all "google.co.nz", "google.nl", etc. as "Google".
		return "Google", true
	}
	if g, ok := groups[refURL.Host]; ok {
		return g, true
	}

	// Useful: https://lobste.rs/s/tslw6k/why_i_m_still_using_jquery_2019
	// Not really: https://lobste.rs/newest/page/8, https://lobste.rs/page/7
	//             https://lobste.rs/search, https://lobste.rs/t/javascript
	if refURL.Host == "lobste.rs" && !strings.HasPrefix(refURL.Path, "/s/") {
		return "lobste.rs", false
	}
	if refURL.Host == "gambe.ro" && !strings.HasPrefix(refURL.Path, "/s/") {
		return "lobste.rs", false
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
		case strings.HasSuffix(refURL.Path, "/search"):
			refURL.Path = refURL.Path[:len(refURL.Path)-7]
		case strings.HasSuffix(refURL.Path, ".compact"):
			refURL.Path = refURL.Path[:len(refURL.Path)-8]
		}
	}

	// Linking https://t.co/c3MITw38Yq isn't too useful as that will link back
	// to the page, so link to the Tweet instead.
	if refURL.Host == "t.co" {
		return "twitter.com/search?q=https%3A%2F%2Ft.co" +
			url.QueryEscape(refURL.Path), false
	}

	// Clean query parameters.
	i := strings.Index(ref, "?")
	if i == -1 {
		// No parameters so no work.
		return strings.TrimLeft(refURL.String(), "/"), false
	}

	q := refURL.Query()
	refURL.RawQuery = ""

	// Google analytics tracking parameters.
	q.Del("utm_source")
	q.Del("utm_medium")
	q.Del("utm_campaign")
	q.Del("utm_term")

	if len(q) == 0 {
		return refURL.String()[2:], false
	}
	return refURL.String()[2:], false
}

func (h *Hit) cleanPath(ctx context.Context) {
	if h.Event {
		h.Path = strings.TrimLeft(h.Path, "/")
		return
	}

	// Normalize the path when accessed from e.g. offline storage or internet
	// archive.
	{
		// Some offline reader thing.
		// /storage/emulated/[..]/Curl_to_shell_isn_t_so_bad2019-11-09-11-07-58/curl-to-sh.html
		if strings.HasPrefix(h.Path, "/storage/emulated/0/Android/data/jonas.tool.saveForOffline/files/") {
			h.Path = h.Path[65:]
			if s := strings.IndexRune(h.Path, '/'); s > -1 {
				h.Path = h.Path[s:]
			}
		}

		// Internet archive.
		// /web/20200104233523/https://www.arp242.net/tmux.html
		if strings.HasPrefix(h.Path, "/web/20") {
			u, err := url.Parse(h.Path[20:])
			if err == nil {
				h.Path = u.Path
				if q := u.Query().Encode(); q != "" {
					h.Path += "?" + q
				}
			}
		}
	}

	// Remove various tracking query parameters.
	{
		h.Path = strings.TrimRight(h.Path, "?&")
		if !strings.Contains(h.Path, "?") { // No query parameters.
			return
		}

		u, err := url.Parse(h.Path)
		if err != nil {
			return
		}
		q := u.Query()

		q.Del("fbclid") // Magic undocumented Facebook tracking parameter.
		q.Del("ref")    // ProductHunt and a few others.
		q.Del("mc_cid") // MailChimp
		q.Del("mc_eid")
		for k := range q { // Google tracking parameters.
			if strings.HasPrefix(k, "utm_") {
				q.Del(k)
			}
		}

		// Some WeChat tracking thing; see e.g:
		// https://translate.google.com/translate?sl=auto&tl=en&u=https%3A%2F%2Fsheshui.me%2Fblogs%2Fexplain-wechat-nsukey-url
		// https://translate.google.com/translate?sl=auto&tl=en&u=https%3A%2F%2Fwww.v2ex.com%2Ft%2F312163
		q.Del("nsukey")
		q.Del("isappinstalled")
		if q.Get("from") == "singlemessage" || q.Get("from") == "groupmessage" {
			q.Del("from")
		}

		u.RawQuery = q.Encode()
		h.Path = u.String()
	}
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
	site := MustGetSite(ctx)
	h.Site = site.ID

	if h.CreatedAt.IsZero() {
		h.CreatedAt = Now()
	}

	h.cleanPath(ctx)

	// Set campaign.
	if !h.Event && h.Query != "" {
		if h.Query[0] != '?' {
			h.Query = "?" + h.Query
		}
		u, err := url.Parse(h.Query)
		if err != nil {
			return
		}
		q := u.Query()

		for _, c := range site.Settings.Campaigns {
			if _, ok := q[c]; ok {
				h.Ref = q.Get(c)
				h.RefURL = nil
				h.RefScheme = RefSchemeCampaign
				break
			}
		}
	}

	if h.Ref != "" && h.RefURL != nil {
		if h.RefURL.Scheme == "http" || h.RefURL.Scheme == "https" {
			h.RefScheme = RefSchemeHTTP
		} else {
			h.RefScheme = RefSchemeOther
		}

		var generated bool
		h.Ref, generated = cleanRefURL(h.Ref, h.RefURL)
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

	v.Len("path", h.Path, 1, 2048)
	v.Len("title", h.Title, 0, 1024)
	v.Len("ref", h.Ref, 0, 2048)
	v.Len("browser", h.Browser, 0, 512)

	return v.ErrorOrNil()
}

type Hits []Hit

// List all hits for a site, including bot requests.
func (h *Hits) List(ctx context.Context, limit, paginate int64) (int64, error) {
	if limit == 0 || limit > 5000 {
		limit = 5000
	}

	err := zdb.MustGet(ctx).SelectContext(ctx, h,
		`select * from hits where site=$1 and id>$2 order by id asc limit $3`,
		MustGetSite(ctx).ID, paginate, limit)

	last := paginate
	if len(*h) > 0 {
		hh := *h
		last = hh[len(hh)-1].ID
	}

	return last, errors.Wrap(err, "Hits.List")
}

// Count the number of pageviews.
func (h *Hits) Count(ctx context.Context) (int64, error) {
	var c int64
	err := zdb.MustGet(ctx).GetContext(ctx, &c,
		`select coalesce(sum(total), 0) from hit_counts where site=$1`,
		MustGetSite(ctx).ID)
	return c, errors.Wrap(err, "Hits.Count")
}

// Purge all paths matching the like pattern.
func (h *Hits) Purge(ctx context.Context, path string, matchTitle bool) error {
	query := `/* Hits.Purge */
		delete from %s where site=$1 and lower(path) like lower($2)`
	if matchTitle {
		query += ` and lower(title) like lower($2) `
	}

	return zdb.TX(ctx, func(ctx context.Context, tx zdb.DB) error {
		site := MustGetSite(ctx).ID

		for _, t := range []string{"hits", "hit_stats", "hit_counts"} {
			_, err := tx.ExecContext(ctx, fmt.Sprintf(query, t), site, path)
			if err != nil {
				return errors.Wrapf(err, "Hits.Purge %s", t)
			}
		}
		_, err := tx.ExecContext(ctx, `/* Hits.Purge */
			delete from ref_counts where site=$1 and lower(path) like lower($2)`,
			site, path)
		if err != nil {
			return errors.Wrap(err, "Hits.Purge ref_counts")
		}

		// Delete all other stats as well if there's nothing left: not much use
		// for it.
		var check Hits
		n, err := check.Count(ctx)
		if err == nil && n == 0 {
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
	Day          string
	Hourly       []int
	HourlyUnique []int
	Daily        int
	DailyUnique  int
}

type HitStat struct {
	Count       int      `db:"count"`
	CountUnique int      `db:"count_unique"`
	Path        string   `db:"path"`
	Event       zdb.Bool `db:"event"`
	Title       string   `db:"title"`
	RefScheme   *string  `db:"ref_scheme"`
	Max         int
	Stats       []Stat
}

type HitStats []HitStat

// ListPathsLike lists all paths matching the like pattern.
func (h *HitStats) ListPathsLike(ctx context.Context, path string, matchTitle bool) error {
	t := ""
	if matchTitle {
		t = " or lower(title) like lower($2) "
	}

	err := zdb.MustGet(ctx).SelectContext(ctx, h, `
		select path, title, sum(total) as count from hit_counts
		where site=$1 and (lower(path) like lower($2) `+t+`)
		group by path, title
		order by count desc
	`, MustGetSite(ctx).ID, path)
	return errors.Wrap(err, "Hits.ListPathsLike")
}

// TODO: should be Stat, but that's already taken and don't want to rename
// everything right now.
type StatT struct {
	Name        string  `db:"name"`
	Count       int     `db:"count"`
	CountUnique int     `db:"count_unique"`
	RefScheme   *string `db:"ref_scheme"`
}

type Stats []StatT

// ByRef lists all paths by reference.
func (h *Stats) ByRef(ctx context.Context, start, end time.Time, ref string, limit int) (int, error) {
	err := zdb.MustGet(ctx).SelectContext(ctx, h, `/* Stats.ByRef */
		select
			path as name,
			coalesce(sum(total), 0) as count,
			coalesce(sum(total_unique), 0) as count_unique
		from ref_counts where
			site=$1 and
			hour>=$2 and
			hour<=$3 and
			ref = $4
		group by path
		order by count desc
		limit $5`,
		MustGetSite(ctx).ID, start.Format(zdb.Date), end.Format(zdb.Date), ref, limit)

	var total int
	for _, b := range *h {
		total += b.Count
	}
	return total, errors.Wrap(err, "HitStats.ByRef")
}

// ListBrowsers lists all browser statistics for the given time period.
func (h *Stats) ListBrowsers(ctx context.Context, start, end time.Time) (int, error) {
	err := zdb.MustGet(ctx).SelectContext(ctx, h, `/* Stats.ListBrowsers */
		select
			browser as name,
			sum(count) as count,
			sum(count_unique) as count_unique
		from browser_stats
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
			sum(count) as count,
			sum(count_unique) as count_unique
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

// ListSystems lists OS statistics for the given time period.
func (h *Stats) ListSystems(ctx context.Context, start, end time.Time) (int, error) {
	err := zdb.MustGet(ctx).SelectContext(ctx, h, `/* Stats.ListSystem */
		select
			system as name,
			sum(count) as count,
			sum(count_unique) as count_unique
		from system_stats
		where site=$1 and day >= $2 and day <= $3
		group by system
		order by count desc
	`, MustGetSite(ctx).ID, start.Format("2006-01-02"), end.Format("2006-01-02"))
	if err != nil {
		return 0, errors.Wrap(err, "Stats.OS")
	}

	var total int
	for _, b := range *h {
		total += b.Count
	}

	return total, nil
}

// ListSystem lists all the versions for one system.
func (h *Stats) ListSystem(ctx context.Context, system string, start, end time.Time) (int, error) {
	err := zdb.MustGet(ctx).SelectContext(ctx, h, `
		select
			system || ' ' || version as name,
			sum(count) as count,
			sum(count_unique) as count_unique
		from system_stats
		where site=$1 and day >= $2 and day <= $3 and lower(system)=lower($4)
		group by system, version
		order by count desc
	`, MustGetSite(ctx).ID, start.Format("2006-01-02"), end.Format("2006-01-02"), system)
	if err != nil {
		return 0, errors.Wrap(err, "Stats.ListSystem")
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
	err := zdb.MustGet(ctx).SelectContext(ctx, h, `/* Stats.ListSizes */
		select
			width as name,
			sum(count) as count,
			sum(count_unique) as count_unique
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
		{Name: sizePhones, Count: 0, CountUnique: 0},
		{Name: sizeLargePhones, Count: 0, CountUnique: 0},
		{Name: sizeTablets, Count: 0, CountUnique: 0},
		{Name: sizeDesktop, Count: 0, CountUnique: 0},
		{Name: sizeDesktopHD, Count: 0, CountUnique: 0},
		{Name: sizeUnknown, Count: 0, CountUnique: 0},
	}

	hh := *h
	var count int
	for i := range hh {
		count += hh[i].Count

		x, _ := strconv.ParseInt(hh[i].Name, 10, 16)
		switch {
		case x == 0:
			ns[5].Count += hh[i].Count
			ns[5].CountUnique += hh[i].CountUnique
		case x <= 384:
			ns[0].Count += hh[i].Count
			ns[0].CountUnique += hh[i].CountUnique
		case x <= 1024:
			ns[1].Count += hh[i].Count
			ns[1].CountUnique += hh[i].CountUnique
		case x <= 1440:
			ns[2].Count += hh[i].Count
			ns[2].CountUnique += hh[i].CountUnique
		case x <= 1920:
			ns[3].Count += hh[i].Count
			ns[3].CountUnique += hh[i].CountUnique
		default:
			ns[4].Count += hh[i].Count
			ns[4].CountUnique += hh[i].CountUnique
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
		return 0, errors.Errorf("Stats.ListSizes: invalid value for name: %#v", name)
	}

	err := zdb.MustGet(ctx).SelectContext(ctx, h, fmt.Sprintf(`/* Stats.ListLocations */
		select
			width as name,
			sum(count) as count,
			sum(count_unique) as count_unique
		from size_stats
		where
			site=$1 and day >= $2 and day <= $3 and
			%s
		group by width
	`, where), MustGetSite(ctx).ID, start.Format("2006-01-02"), end.Format("2006-01-02"))
	if err != nil {
		return 0, errors.Wrap(err, "Stats.ListSize")
	}

	grouped := make(map[string]int)
	groupedUnique := make(map[string]int)
	hh := *h
	for i := range hh {
		grouped[fmt.Sprintf("↔ %spx", hh[i].Name)] += hh[i].Count
		groupedUnique[fmt.Sprintf("↔ %spx", hh[i].Name)] += hh[i].CountUnique
	}

	ns := Stats{}
	total := 0
	for width, count := range grouped {
		total += count
		ns = append(ns, StatT{
			Name:        width,
			Count:       count,
			CountUnique: groupedUnique[width],
		})
	}
	sort.Slice(ns, func(i int, j int) bool { return ns[i].Count > ns[j].Count })
	*h = ns

	return total, nil
}

// ListLocations lists all location statistics for the given time period.
func (h *Stats) ListLocations(ctx context.Context, start, end time.Time) (int, error) {
	err := zdb.MustGet(ctx).SelectContext(ctx, h, `/* Stats.ListLocations */
		select
			iso_3166_1.name as name,
			sum(count) as count,
			sum(count_unique) as count_unique
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

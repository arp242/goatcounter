// Copyright © 2019 Martin Tournoij <martin@arp242.net>
// This file is part of GoatCounter and published under the terms of the EUPL
// v1.2, which can be found in the LICENSE file or at http://eupl12.zgo.at

package goatcounter

import (
	"bytes"
	"context"
	"fmt"
	"net/url"
	"strings"
	"text/tabwriter"
	"time"

	"zgo.at/errors"
	"zgo.at/zdb"
	"zgo.at/zlog"
	"zgo.at/zvalidate"
)

func ptr(s string) *string { return &s }

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

type StatT struct {
	// TODO: should be Stat, but that's already taken and don't want to rename
	// everything right now.
	Name        string  `db:"name"`
	Count       int     `db:"count"`
	CountUnique int     `db:"count_unique"`
	RefScheme   *string `db:"ref_scheme"`
}

type Stats struct {
	More  bool
	Stats []StatT
}

// ByRef lists all paths by reference.
func (h *Stats) ByRef(ctx context.Context, start, end time.Time, ref string) error {
	err := zdb.MustGet(ctx).SelectContext(ctx, &h.Stats, `/* Stats.ByRef */
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
		limit 10`,
		MustGetSite(ctx).ID, start.Format(zdb.Date), end.Format(zdb.Date), ref)

	return errors.Wrap(err, "Stats.ByRef")
}

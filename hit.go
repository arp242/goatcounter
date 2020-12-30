// Copyright © 2019 Martin Tournoij – This file is part of GoatCounter and
// published under the terms of a slightly modified EUPL v1.2 license, which can
// be found in the LICENSE file or at https://license.goatcounter.com

package goatcounter

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
	"zgo.at/errors"
	"zgo.at/zdb"
	"zgo.at/zstd/zint"
	"zgo.at/zvalidate"
)

func ptr(s string) *string { return &s }
func unref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

type Hit struct {
	ID          int64        `db:"hit_id" json:"-"`
	Site        int64        `db:"site_id" json:"-"`
	PathID      int64        `db:"path_id" json:"-"`
	UserAgentID *int64       `db:"user_agent_id" json:"-"`
	Session     zint.Uint128 `db:"session" json:"-"`

	Path  string     `db:"-" json:"p,omitempty"`
	Title string     `db:"-" json:"t,omitempty"`
	Ref   string     `db:"ref" json:"r,omitempty"`
	Event zdb.Bool   `db:"-" json:"e,omitempty"`
	Size  zdb.Floats `db:"size" json:"s,omitempty"`
	Query string     `db:"-" json:"q,omitempty"`
	Bot   int        `db:"bot" json:"b,omitempty"`

	RefScheme       *string   `db:"ref_scheme" json:"-"`
	UserAgentHeader string    `db:"-" json:"-"`
	Location        string    `db:"location" json:"-"`
	FirstVisit      zdb.Bool  `db:"first_visit" json:"-"`
	CreatedAt       time.Time `db:"created_at" json:"-"`

	RefURL *url.URL `db:"-" json:"-"`   // Parsed Ref
	Random string   `db:"-" json:"rnd"` // Browser cache buster, as they don't always listen to Cache-Control

	// Some values we need to pass from the HTTP handler to memstore
	RemoteAddr    string `db:"-" json:"-"`
	UserSessionID string `db:"-" json:"-"`
	BrowserID     int64  `db:"-" json:"-"`
	SystemID      int64  `db:"-" json:"-"`
}

func (h *Hit) Ignore() bool {
	// kproxy.com; not easy to get the original path, so just ignore it.
	if strings.HasPrefix(h.Path, "/servlet/redirect.srv/") {
		return true
	}
	// Almost certainly some broken HTML or whatnot.
	if strings.Contains(h.Path, "<html>") || strings.Contains(h.Path, "<HTML>") {
		return true
	}
	// Don't record favicon from logfiles.
	if h.Path == "/favicon.ico" {
		return true
	}

	return false
}

func (h *Hit) cleanPath(ctx context.Context) {
	h.Path = strings.TrimSpace(h.Path)
	if h.Event {
		h.Path = strings.TrimLeft(h.Path, "/")
		return
	}

	if h.Path == "" { // Don't fill empty path to "/"
		return
	}

	h.Path = "/" + strings.Trim(h.Path, "/")

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
				if h.Path == "" {
					h.Path = "/"
				}
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
		q.Del("gclid") // AdWords click ID

		// Some WeChat tracking thing; see e.g:
		// https://translate.google.com/translate?sl=auto&tl=en&u=https%3A%2F%2Fsheshui.me%2Fblogs%2Fexplain-wechat-nsukey-url
		// https://translate.google.com/translate?sl=auto&tl=en&u=https%3A%2F%2Fwww.v2ex.com%2Ft%2F312163
		q.Del("nsukey")
		q.Del("isappinstalled")
		if q.Get("from") == "singlemessage" || q.Get("from") == "groupmessage" {
			q.Del("from")
		}

		// Cloudflare
		q.Del("__cf_chl_captcha_tk__")
		q.Del("__cf_chl_jschl_tk__")

		u.RawQuery = q.Encode()
		h.Path = u.String()
	}
}

// Defaults sets fields to default values, unless they're already set.
func (h *Hit) Defaults(ctx context.Context, initial bool) error {
	site := MustGetSite(ctx)
	h.Site = site.ID

	if h.CreatedAt.IsZero() {
		h.CreatedAt = Now()
	}

	if h.Event {
		h.Path = strings.TrimLeft(h.Path, "/")
		// In case people send "/" as the event path.
		if h.Path == "" {
			h.Path = "(no event name)"
		}
	} else {
		h.cleanPath(ctx)
	}

	// Set campaign.
	if !h.Event && h.Query != "" {
		if h.Query[0] != '?' {
			h.Query = "?" + h.Query
		}
		u, err := url.Parse(h.Query)
		if err != nil {
			return errors.Wrap(err, "Hit.Defaults")
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

	if h.RefScheme == nil && h.Ref != "" && h.RefURL != nil {
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

	if initial {
		return nil
	}

	// Get or insert path.
	path := Path{Path: h.Path, Title: h.Title, Event: h.Event}
	err := path.GetOrInsert(ctx)
	if err != nil {
		return errors.Wrap(err, "Hit.Defaults")
	}
	h.PathID = path.ID

	// Get or insert user_agent
	if site.Settings.Collect.Has(CollectUserAgent) {
		ua := UserAgent{UserAgent: h.UserAgentHeader}
		err = ua.GetOrInsert(ctx)
		if err != nil {
			return errors.Wrap(err, "Hit.Defaults")
		}
		h.UserAgentID = &ua.ID
		h.BrowserID = ua.BrowserID
		h.SystemID = ua.SystemID
	}

	return nil
}

// Validate the object.
func (h *Hit) Validate(ctx context.Context, initial bool) error {
	v := zvalidate.New()

	v.Required("site", h.Site)
	//v.Required("session", h.Session)
	v.Required("created_at", h.CreatedAt)
	v.UTF8("ref", h.Ref)
	v.Len("ref", h.Ref, 0, 2048)

	// Small margin as client's clocks may not be 100% accurate.
	if h.CreatedAt.After(Now().Add(5 * time.Second)) {
		v.Append("created_at", "in the future")
	}

	if initial {
		v.Required("path", h.Path)
		v.UTF8("path", h.Path)
		v.UTF8("title", h.Title)
		v.UTF8("user_agent_header", h.UserAgentHeader)
		v.Len("path", h.Path, 1, 2048)
		v.Len("title", h.Title, 0, 1024)
		v.Len("user_agent_header", h.UserAgentHeader, 0, 512)
	} else {
		v.Required("path_id", h.PathID)

		if MustGetSite(ctx).Settings.Collect.Has(CollectUserAgent) {
			v.Required("user_agent_id", h.UserAgentID)
			v.Required("browser_id", h.BrowserID)
			v.Required("system_id", h.SystemID)
		}
	}

	return v.ErrorOrNil()
}

type Hits []Hit

// TestList lists all hits, for all sites, with browser_id, system_id, and paths
// set.
//
// This is mostly intended for tests.
func (h *Hits) TestList(ctx context.Context, siteOnly bool) error {
	var hh []struct {
		Hit
		B int64    `db:"browser_id"`
		S int64    `db:"system_id"`
		P string   `db:"path"`
		T string   `db:"title"`
		E zdb.Bool `db:"event"`
	}

	err := zdb.Select(ctx, &hh, `/* Hits.TestList */
		select
			hits.*,
			user_agents.browser_id,
			user_agents.system_id,
			paths.path,
			paths.title,
			paths.event
		from hits
		join user_agents using (user_agent_id)
		join paths using (path_id)
		{{:site_only where hits.site_id = :site}}
		order by hit_id asc`,
		zdb.A{
			"site":      MustGetSite(ctx).ID,
			"site_only": siteOnly,
		})
	if err != nil {
		return errors.Wrap(err, "Hits.TestList")
	}

	for _, x := range hh {
		x.Hit.BrowserID = x.B
		x.Hit.SystemID = x.S
		x.Hit.Path = x.P
		x.Hit.Title = x.T
		x.Hit.Event = x.E

		*h = append(*h, x.Hit)
	}
	return nil
}

// Count the number of pageviews.
func (h *Hits) Count(ctx context.Context) (int64, error) {
	var c int64
	err := zdb.Get(ctx, &c,
		`select coalesce(sum(total), 0) from hit_counts where site_id=$1`,
		MustGetSite(ctx).ID)
	return c, errors.Wrap(err, "Hits.Count")
}

// Purge all paths.
func (h *Hits) Purge(ctx context.Context, pathIDs []int64) error {
	query := `/* Hits.Purge */
		delete from %s where site_id=? and path_id in (?)`

	return zdb.TX(ctx, func(ctx context.Context) error {
		site := MustGetSite(ctx).ID

		for _, t := range append(statTables, "hit_counts", "ref_counts", "hits", "paths") {
			query, args, err := sqlx.In(fmt.Sprintf(query, t), site, pathIDs)
			if err != nil {
				return errors.Wrapf(err, "Hits.Purge %s", t)
			}
			err = zdb.Exec(ctx, query, args...)
			if err != nil {
				return errors.Wrapf(err, "Hits.Purge %s", t)
			}
		}

		MustGetSite(ctx).ClearCache(true)
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
	PathID      int64    `db:"path_id"`
	Path        string   `db:"path"`
	Event       zdb.Bool `db:"event"`
	Title       string   `db:"title"`
	RefScheme   *string  `db:"ref_scheme"`
	Max         int
	Stats       []Stat
}

type HitStats []HitStat

// ListPathsLike lists all paths matching the like pattern.
func (h *HitStats) ListPathsLike(ctx context.Context, search string, matchTitle bool) error {
	err := zdb.Select(ctx, h, `/* HitStats.ListPathsLike */
		with x as (
			select path_id, path, title from paths
			where site_id = :site and
			(lower(path) like lower(:search) {{:match_title or lower(title) like lower(:search)}})
		)
		select
			path_id, path, title,
			sum(total) as count
		from hit_counts
		join x using(path_id)
		where site_id = :site
		group by path_id, path, title
		order by count desc`,
		zdb.A{
			"site":        MustGetSite(ctx).ID,
			"search":      search,
			"match_title": matchTitle,
		})
	return errors.Wrap(err, "Hits.ListPathsLike")
}

// PathCountUnique gets the total_unique for one path.
func (h *HitStats) PathCountUnique(ctx context.Context, path string) error {
	err := zdb.Select(ctx, h, `/* HitStats.PathCountUnique */
		with x as (
			select path_id, path from paths
			where site_id = :site and lower(path) = lower(:path)
		)
		select
			path,
			coalesce(sum(
				(select sum(total_unique) from hit_counts where site_id = :site and path_id = x.path_id)
			), 0) as count_unique
		from x
		group by path`,
		zdb.A{
			"site": MustGetSite(ctx).ID,
			"path": path,
		})
	return errors.Wrap(err, "HitStats.PathCountUnique")
}

// SiteTotalUnique gets the total_unique for all paths.
func (h *HitStats) SiteTotalUnique(ctx context.Context) error {
	err := zdb.Select(ctx, h, `/* *HitStats.SiteTotalUnique */
		select sum(total_unique) as count_unique from hit_counts
		where site_id=$1`, MustGetSite(ctx).ID)
	return errors.Wrap(err, "HitStats.SiteTotalUnique")
}

type StatT struct {
	// TODO: should be Stat, but that's already taken and don't want to rename
	// everything right now.
	ID          string  `db:"id"`
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
func (h *Stats) ByRef(ctx context.Context, start, end time.Time, pathFilter []int64, ref string) error {
	err := zdb.Select(ctx, &h.Stats, `/* Stats.ByRef */
		with x as (
			select
				path_id,
				coalesce(sum(total), 0) as count,
				coalesce(sum(total_unique), 0) as count_unique
			from ref_counts
			where
				site_id=:site and hour>=:start and hour<=:end and
				{{:filter path_id in (:filter) and}}
				ref=:ref
			group by path_id
			order by count desc
			limit 10
		)
		select
			paths.path as name,
			x.count,
			x.count_unique
		from x
		join paths using(path_id)`,
		zdb.A{
			"site":   MustGetSite(ctx).ID,
			"start":  start.Format(zdb.Date),
			"end":    end.Format(zdb.Date),
			"filter": pathFilter,
			"ref":    ref,
		})

	return errors.Wrap(err, "Stats.ByRef")
}

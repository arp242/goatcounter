// Copyright © Martin Tournoij – This file is part of GoatCounter and published
// under the terms of a slightly modified EUPL v1.2 license, which can be found
// in the LICENSE file or at https://license.goatcounter.com

package goatcounter

import (
	"context"
	"fmt"
	"net/url"
	"slices"
	"strings"
	"time"

	"zgo.at/errors"
	"zgo.at/zdb"
	"zgo.at/zstd/zbool"
	"zgo.at/zstd/zint"
	"zgo.at/zstd/ztime"
)

type Hit struct {
	ID         int64        `db:"hit_id" json:"-"`
	Site       int64        `db:"site_id" json:"-"`
	PathID     int64        `db:"path_id" json:"-"`
	RefID      int64        `db:"ref_id" json:"-"`
	SizeID     *int64       `db:"size_id" json:"-"`
	BrowserID  int64        `db:"browser_id" json:"-"`
	SystemID   int64        `db:"system_id" json:"-"`
	CampaignID *int64       `db:"campaign" json:"-"`
	Session    zint.Uint128 `db:"session" json:"-"`

	Path  string     `db:"-" json:"p,omitempty"`
	Title string     `db:"-" json:"t,omitempty"`
	Ref   string     `db:"-" json:"r,omitempty"`
	Event zbool.Bool `db:"-" json:"e,omitempty"`
	Size  Floats     `db:"-" json:"s,omitempty"`
	Query string     `db:"-" json:"q,omitempty"`
	Bot   int        `db:"bot" json:"b,omitempty"`

	RefScheme       *string    `db:"ref_scheme" json:"-"`
	UserAgentHeader string     `db:"-" json:"-"`
	Location        string     `db:"location" json:"-"`
	Language        *string    `db:"language" json:"-"`
	FirstVisit      zbool.Bool `db:"first_visit" json:"-"`
	CreatedAt       time.Time  `db:"created_at" json:"-"`

	RefURL *url.URL `db:"-" json:"-"`   // Parsed Ref
	Random string   `db:"-" json:"rnd"` // Browser cache buster, as they don't always listen to Cache-Control

	// Some values we need to pass from the HTTP handler to memstore
	RemoteAddr    string `db:"-" json:"-"`
	UserSessionID string `db:"-" json:"-"`

	NoStore   bool `db:"-" json:"-"` // Don't store in hits (still store in stats).
	noProcess bool `db:"-" json:"-"` // Don't process in memstore; for merging paths.
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

		// Added by Weibo.cn (a sort of Chinese Twitter), with a random ID:
		//   /?continueFlag=4020a77be9019cf14fefc373267aa46e
		//   /?continueFlag=c397418f4346f293408b311b1bc819d4
		// Presumably a tracking thing?
		q.Del("continueFlag")

		q.Del("_x_tr_sl") // Google translate
		q.Del("_x_tr_hl")
		q.Del("_x_tr_pto")
		if q.Has("_x_tr_tl") { // Rename the destination language.
			q.Set("translate-to", q.Get("_x_tr_tl"))
			q.Del("_x_tr_tl")
		}

		u.RawQuery = q.Encode()
		h.Path = "/" + strings.Trim(u.String(), "/")
	}
}

// Defaults sets fields to default values, unless they're already set.
func (h *Hit) Defaults(ctx context.Context, initial bool) error {
	site := MustGetSite(ctx)
	h.Site = site.ID

	if h.CreatedAt.IsZero() {
		h.CreatedAt = ztime.Now()
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

		// Get referral from query
		for _, c := range []string{"utm_source", "ref", "src", "source"} {
			v := strings.TrimSpace(q.Get(c))
			if v == "" {
				continue
			}

			h.Ref = v
			h.RefURL = nil
			h.RefScheme = RefSchemeCampaign
			break
		}

		// Get campaign.
		for _, c := range []string{"utm_campaign", "campaign"} {
			v := strings.TrimSpace(q.Get(c))
			if v == "" {
				continue
			}

			c := Campaign{Name: v}
			err := c.ByName(ctx, c.Name)
			if err != nil && !zdb.ErrNoRows(err) {
				return errors.Wrap(err, "Hit.Defaults")
			}

			if zdb.ErrNoRows(err) {
				err := c.Insert(ctx)
				if err != nil {
					return errors.Wrap(err, "Hit.Defaults")
				}
			}
			h.CampaignID = &c.ID
			h.RefScheme = RefSchemeCampaign
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

	// Get or insert ref.
	ref := Ref{Ref: h.Ref, RefScheme: h.RefScheme}
	err = ref.GetOrInsert(ctx)
	if err != nil {
		return errors.Wrap(err, "Hit.Defaults")
	}
	h.RefID = ref.ID

	// Get or insert size.
	if site.Settings.Collect.Has(CollectScreenSize) {
		var size Size
		err = size.GetOrInsert(ctx, h.Size)
		if err != nil {
			return errors.Wrap(err, "Hit.Defaults")
		}
		h.SizeID = &size.ID
	}

	// Get or insert browser and system.
	if site.Settings.Collect.Has(CollectUserAgent) {
		ua := UserAgent{UserAgent: h.UserAgentHeader}
		err = ua.GetOrInsert(ctx)
		if err != nil {
			return errors.Wrap(err, "Hit.Defaults")
		}
		h.BrowserID = ua.BrowserID
		h.SystemID = ua.SystemID
	}

	return nil
}

// Validate the object.
func (h *Hit) Validate(ctx context.Context, initial bool) error {
	v := NewValidate(ctx)

	v.Required("site", h.Site)
	//v.Required("session", h.Session)
	v.Required("created_at", h.CreatedAt)
	v.UTF8("ref", h.Ref)
	v.Len("ref", h.Ref, 0, 2048)

	// Small margin as client's clocks may not be 100% accurate.
	if h.CreatedAt.After(ztime.Now().Add(5 * time.Second)) {
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
// This is intended for tests.
func (h *Hits) TestList(ctx context.Context, siteOnly bool) error {
	var hh []struct {
		Hit
		B    int64      `db:"browser_id"`
		S    int64      `db:"system_id"`
		P    string     `db:"path"`
		T    string     `db:"title"`
		E    zbool.Bool `db:"event"`
		R    string     `db:"ref"`
		Size Floats     `db:"size"`
	}

	err := zdb.Select(ctx, &hh, `/* Hits.TestList */
		select
			hits.*,
			browser_id,
			system_id,
			paths.path,
			paths.title,
			paths.event,
			refs.ref,
			sizes.size
		from hits
		join paths using (path_id)
		left join refs  using (ref_id)
		left join sizes using (size_id)
		{{:site_only where hits.site_id = :site}}
		order by hit_id asc`,
		map[string]any{
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
		x.Hit.Ref = x.R
		x.Hit.Size = x.Size

		*h = append(*h, x.Hit)
	}
	return nil
}

// Purge the given paths.
func (h *Hits) Purge(ctx context.Context, pathIDs []int64) error {
	query := `/* Hits.Purge */
		delete from %s where site_id=? and path_id in (?)`

	return zdb.TX(ctx, func(ctx context.Context) error {
		site := MustGetSite(ctx).ID

		for _, t := range append(statTables, "hit_counts", "ref_counts", "hits", "paths") {
			err := zdb.Exec(ctx, fmt.Sprintf(query, t), site, pathIDs)
			if err != nil {
				return errors.Wrapf(err, "Hits.Purge %s", t)
			}
		}

		MustGetSite(ctx).ClearCache(ctx, true)
		return nil
	})
}

// Merge the given paths.
func (h *Hits) Merge(ctx context.Context, dst int64, pathIDs []int64) error {
	// Shouldn't happen, but just in case.
	pathIDs = slices.DeleteFunc(pathIDs, func(p int64) bool { return p == dst })

	site := MustGetSite(ctx).ID

	err := (&Path{}).ByID(ctx, dst) // Ensure this site owns the path.
	if err != nil {
		return errors.Wrap(err, "Hits.Merge")
	}

	// Push back to lot to memstore to re-add it again, and then just call
	// Purge() to delete the old ones.
	err = zdb.Select(ctx, h, `select * from hits where site_id=? and path_id in (?)`, site, pathIDs)
	if err != nil {
		return errors.Wrap(err, "Hits.Merge")
	}
	hh := *h
	for i := range hh {
		hh[i].PathID = dst
		hh[i].noProcess = true
	}

	err = errors.Wrap(h.Purge(ctx, pathIDs), "Hits.Merge")
	if err != nil {
		return errors.Wrap(err, "Hits.Merge")
	}

	// Only push back if delete worked.
	Memstore.Append(hh...)
	return nil
}

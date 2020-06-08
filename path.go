// Copyright © 2019 Martin Tournoij <martin@arp242.net>
// This file is part of GoatCounter and published under the terms of the EUPL
// v1.2, which can be found in the LICENSE file or at http://eupl12.zgo.at

package goatcounter

import (
	"context"
	"net/url"
	"strings"

	"zgo.at/errors"
	"zgo.at/goatcounter/cfg"
	"zgo.at/zdb"
	"zgo.at/zvalidate"
)

type Path struct {
	ID   int64 `db:"path_id"`
	Site int64 `db:"site"`

	Path  string   `db:"path"`
	Title string   `db:"title"`
	Event zdb.Bool `db:"event"`
}

func (h *Path) cleanPath(ctx context.Context) {
	if h.Event {
		h.Path = strings.TrimLeft(h.Path, "/")
		return
	} else {
		h.Path = "/" + strings.Trim(h.Path, "/")
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

func (p *Path) Defaults(ctx context.Context) {
	p.cleanPath(ctx)
}

func (p *Path) Validate(ctx context.Context) error {
	v := zvalidate.New()

	v.UTF8("path", p.Path)
	v.UTF8("title", p.Title)
	v.Len("path", p.Path, 1, 2048)
	v.Len("title", p.Title, 0, 1024)

	return v.ErrorOrNil()
}

// TODO: update title once a day or something?
func (p *Path) GetOrInsert(ctx context.Context) error {
	db := zdb.MustGet(ctx)
	site := MustGetSite(ctx)

	p.Defaults(ctx)
	err := p.Validate(ctx)
	if err != nil {
		return err
	}

	row := db.QueryRowxContext(ctx, `/* Path.GetOrInsert */
		select * from paths
		where site=$1 and lower(path)=lower($2)
		limit 1`,
		site.ID, p.Path)
	if row.Err() != nil {
		return errors.Errorf("Path.GetOrInsert select: %w", row.Err())
	}
	err = row.StructScan(p)
	if err != nil && !zdb.ErrNoRows(err) {
		return errors.Errorf("Path.GetOrInsert select: %w", err)
	}
	if err == nil {
		return nil
	}

	// Insert new row.
	query := `insert into paths (site, path, title) values ($1, $2, $3)`
	args := []interface{}{site.ID, p.Path, p.Title}
	if cfg.PgSQL {
		err = db.GetContext(ctx, &p.ID, query+" returning id", args...)
		return errors.Wrap(err, "Path.GetOrInsert insert")
	}

	r, err := db.ExecContext(ctx, query, args...)
	if err != nil {
		return errors.Errorf("Path.GetOrInsert insert: %w", err)
	}
	p.ID, err = r.LastInsertId()
	if err != nil {
		return errors.Errorf("Path.GetOrInsert insert: %w", err)
	}

	return nil
}

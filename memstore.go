// Copyright © 2019 Martin Tournoij <martin@arp242.net>
// This file is part of GoatCounter and published under the terms of the EUPL
// v1.2, which can be found in the LICENSE file or at http://eupl12.zgo.at

package goatcounter

import (
	"context"
	"net/url"
	"sync"

	"zgo.at/zdb"
	"zgo.at/zdb/bulk"
	"zgo.at/zlog"
)

type ms struct {
	sync.RWMutex
	hits []Hit
}

var Memstore = ms{}

func (m *ms) Append(hits ...Hit) {
	m.Lock()
	m.hits = append(m.hits, hits...)
	m.Unlock()
}

func (m *ms) Len() int {
	m.Lock()
	l := len(m.hits)
	m.Unlock()
	return l
}

func (m *ms) Persist(ctx context.Context) ([]Hit, error) {
	if m.Len() == 0 {
		return nil, nil
	}

	m.Lock()
	hits := make([]Hit, len(m.hits))
	copy(hits, m.hits)
	m.hits = []Hit{}
	m.Unlock()

	sites := make(map[int64]*Site)

	l := zlog.Module("memstore")

	ins := bulk.NewInsert(ctx, "hits", []string{"site", "path", "ref",
		"ref_scheme", "browser", "size", "location", "created_at", "bot",
		"title", "event", "session", "first_visit"})
	for i, h := range hits {
		// Ignore spammers.
		h.RefURL, _ = url.Parse(h.Ref)
		if h.RefURL != nil {
			if _, ok := blacklist[h.RefURL.Host]; ok {
				l.Debugf("blacklisted: %q", h.RefURL.Host)
				continue
			}
		}

		site, ok := sites[h.Site]
		if !ok {
			site = new(Site)
			err := site.ByID(ctx, h.Site)
			if err != nil {
				l.Field("hit", h).Error(err)
				continue
			}
			sites[h.Site] = site
		}
		ctx = WithSite(ctx, site)

		h.Defaults(ctx)
		err := h.Validate(ctx)
		if err != nil {
			l.Field("hit", h).Error(err)
			continue
		}

		// Some values are sanitized in Hit.Defaults(), make sure this is
		// reflected in the hits object too, which matters for the hit_stats
		// generation later.
		hits[i] = h

		ins.Values(h.Site, h.Path, h.Ref, h.RefScheme, h.Browser, h.Size,
			h.Location, h.CreatedAt.Format(zdb.Date), h.Bot, h.Title, h.Event,
			h.Session, h.FirstVisit)
	}

	return hits, ins.Finish()
}

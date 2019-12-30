// Copyright © 2019 Martin Tournoij <martin@arp242.net>
// This file is part of GoatCounter and published under the terms of the EUPL
// v1.2, which can be found in the LICENSE file or at eupl12.zgo.at

package goatcounter

import (
	"context"
	"net/url"
	"sync"

	"github.com/jmoiron/sqlx"
	"zgo.at/zdb"
	"zgo.at/zdb/bulk"
	"zgo.at/zlog"
)

type ms struct {
	sync.RWMutex
	hits []Hit
}

var Memstore = ms{}

func (m *ms) Append(hit ...Hit) {
	m.Lock()
	m.hits = append(m.hits, hit...)
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

	// TODO: this could be a channel.
	m.Lock()
	hits := make([]Hit, len(m.hits))
	copy(hits, m.hits)
	m.hits = []Hit{}
	m.Unlock()

	ins := bulk.NewInsert(ctx, zdb.MustGet(ctx).(*sqlx.DB),
		"hits", []string{"site", "path", "ref", "ref_params", "ref_original",
			"ref_scheme", "browser", "size", "location", "created_at", "count_ref"})
	for i, h := range hits {
		var err error
		h.refURL, err = url.Parse(h.Ref)
		if err != nil {
			zlog.Field("ref", h.Ref).Errorf("could not parse ref: %s", err)
			continue
		}

		// Ignore spammers.
		if _, ok := blacklist[h.refURL.Host]; ok {
			continue
		}

		h.Defaults(ctx)
		err = h.Validate(ctx)
		if err != nil {
			zlog.Error(err)
			continue
		}

		// Some values are sanitized in Hit.Defaults(), make sure this is
		// reflected in the hits object too, which matters for the hit_stats
		// generation later.
		hits[i] = h

		countRef := h.CountRef
		if countRef != "" {
			u, _ := url.Parse(countRef)
			if u != nil {
				countRef = u.Host
			}
		}

		ins.Values(h.Site, h.Path, h.Ref, h.RefParams, h.RefOriginal,
			h.RefScheme, h.Browser, h.Size, h.Location, zdb.Date(h.CreatedAt),
			countRef)
	}

	return hits, ins.Finish()
}

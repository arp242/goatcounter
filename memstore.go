// Copyright © 2019 Martin Tournoij <martin@arp242.net>
// This file is part of GoatCounter and published under the terms of the AGPLv3,
// which can be found in the LICENSE file or at gnu.org/licenses/agpl.html

package goatcounter

import (
	"context"
	"fmt"
	"net/url"
	"sync"

	"github.com/jmoiron/sqlx"
	"zgo.at/zlog"

	"zgo.at/goatcounter/bulk"
)

type ms struct {
	sync.RWMutex
	hits     []Hit
	browsers []Browser
}

var Memstore = ms{}

func (m *ms) Append(hit Hit, browser Browser) {
	m.Lock()
	m.hits = append(m.hits, hit)
	m.browsers = append(m.browsers, browser)
	m.Unlock()
}

func (m *ms) Persist(ctx context.Context) error {
	if len(m.hits) == 0 {
		return nil
	}

	l := zlog.SetDebug("memstore").Module("memstore")

	m.Lock()
	hits := make([]Hit, len(m.hits))
	browsers := make([]Browser, len(m.browsers))
	copy(hits, m.hits)
	copy(browsers, m.browsers)
	m.hits = []Hit{}
	m.browsers = []Browser{}
	m.Unlock()

	ins := bulk.NewInsert(ctx, MustGetDB(ctx).(*sqlx.DB),
		"hits", []string{"site", "path", "ref", "ref_params", "ref_original", "ref_scheme", "created_at"})
	for _, h := range hits {
		var err error
		h.refURL, err = url.Parse(h.Ref)
		if err != nil {
			zlog.Fields(zlog.F{"ref": h.Ref}).Errorf("could not parse ref: %s", err)
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

		ins.Values(h.Site, h.Path, h.Ref, h.RefParams, h.RefOriginal, h.RefScheme, sqlDate(h.CreatedAt))
	}
	err := ins.Finish()
	if err != nil {
		zlog.Error(err)
	}

	ins = bulk.NewInsert(ctx, MustGetDB(ctx).(*sqlx.DB),
		"browsers", []string{"site", "browser", "created_at"})
	for _, b := range browsers {
		b.Defaults(ctx)
		err := b.Validate(ctx)
		if err != nil {
			zlog.Error(err)
			continue
		}

		ins.Values(b.Site, b.Browser, sqlDate(b.CreatedAt))
	}
	err = ins.Finish()
	if err != nil {
		zlog.Error(err)
	}

	l.Since(fmt.Sprintf("persisted %d hits and %d User-Agents", len(hits), len(browsers)))

	return nil
}

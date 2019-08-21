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
	"zgo.at/zhttp/ctxkey"
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

	l := zlog.Debug("memstore").Module("memstore")

	m.Lock()
	hits := make([]Hit, len(m.hits))
	browsers := make([]Browser, len(m.browsers))
	copy(hits, m.hits)
	copy(browsers, m.browsers)
	m.hits = []Hit{}
	m.browsers = []Browser{}
	m.Unlock()

	ins := bulk.NewInsert(ctx, MustGetDB(ctx).(*sqlx.DB),
		"hits", []string{"site", "domain", "path", "ref", "ref_params", "ref_original", "ref_scheme", "created_at"})
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

		sctx := context.WithValue(ctx, ctxkey.Site, &Site{ID: h.Site})
		h.Defaults(sctx)
		err = h.Validate(sctx)
		if err != nil {
			zlog.Error(err)
			continue
		}

		ins.Values(h.Site, h.Domain, h.Path, h.Ref, h.RefParams, h.RefOriginal, h.RefScheme, sqlDate(h.CreatedAt))
	}
	err := ins.Finish()
	if err != nil {
		zlog.Error(err)
	}

	ins = bulk.NewInsert(ctx, MustGetDB(ctx).(*sqlx.DB),
		"browsers", []string{"site", "domain", "browser", "created_at"})
	for _, b := range browsers {
		sctx := context.WithValue(ctx, ctxkey.Site, &Site{ID: b.Site})
		b.Defaults(sctx)
		err := b.Validate(sctx)
		if err != nil {
			zlog.Error(err)
			continue
		}

		ins.Values(b.Site, b.Domain, b.Browser, sqlDate(b.CreatedAt))
	}
	err = ins.Finish()
	if err != nil {
		zlog.Error(err)
	}

	l.Since(fmt.Sprintf("persisted %d hits and %d User-Agents", len(hits), len(browsers)))

	return nil
}

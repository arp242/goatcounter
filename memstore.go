package goatcounter

import (
	"context"
	"sync"

	"zgo.at/zlog"
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

	l := zlog.Module("memstore")

	m.Lock()
	hits := make([]Hit, len(m.hits))
	browsers := make([]Browser, len(m.browsers))
	copy(hits, m.hits)
	copy(browsers, m.browsers)
	m.hits = []Hit{}
	m.browsers = []Browser{}
	m.Unlock()

	l.Printf("persisting %d hits and %d User-Agents", len(hits), len(browsers))
	for _, h := range hits {
		err := h.Insert(ctx)
		if err != nil {
			l.Errorf("inserting hit %v: %s", h, err)
		}
	}

	for _, b := range browsers {
		err := b.Insert(ctx)
		if err != nil {
			l.Errorf("inserting browser %v: %s", b, err)
		}
	}

	return nil
}

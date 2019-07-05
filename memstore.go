package goatcounter

import (
	"context"
	"sync"

	"zgo.at/zlog"
)

type ms struct {
	sync.RWMutex
	hits []Hit
}

var Memstore = ms{}

func (m *ms) Append(hit Hit) {
	m.Lock()
	m.hits = append(m.hits, hit)
	m.Unlock()
}

func (m *ms) Persist(ctx context.Context) error {
	if len(m.hits) == 0 {
		return nil
	}

	l := zlog.Module("memstore")

	m.Lock()
	hits := make([]Hit, len(m.hits))
	copy(hits, m.hits)
	m.hits = []Hit{}
	m.Unlock()

	l.Printf("persisting %d", len(hits))
	for _, h := range hits {
		err := h.Insert(ctx)
		if err != nil {
			l.Error(err)
		}
	}
	l.Print("done")

	return nil
}

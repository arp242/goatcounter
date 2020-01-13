package goatcounter

import (
	"context"
	"sync"
)

type hr struct {
	sync.Mutex
	m map[int64]map[string]int
}

var HitRef = make(map[string]int)

func (h *hr) Increment(site int64, host string) {
	h.Lock()
	h.m[site][host]++
	h.Unlock()
}

func (h *hr) Persist(ctx context.Context) {
}

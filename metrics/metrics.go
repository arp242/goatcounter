// Copyright © Martin Tournoij – This file is part of GoatCounter and published
// under the terms of a slightly modified EUPL v1.2 license, which can be found
// in the LICENSE file or at https://license.goatcounter.com

// Package metrics collects performance metrics.
package metrics

import (
	"sort"
	"sync"
	"time"

	"zgo.at/zstd/ztime"
)

type metrics struct {
	mu    *sync.Mutex
	stats map[string]ztime.Durations
}

var collected = metrics{
	mu:    new(sync.Mutex),
	stats: make(map[string]ztime.Durations, 32),
}

func (m metrics) add(tag string, d time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()

	t, ok := m.stats[tag]
	if !ok {
		t = ztime.NewDurations(32768)
		t.Grow(32768)
	}
	t.Append(d)
	m.stats[tag] = t
}

type Metrics []struct {
	Tag   string
	Times ztime.Durations
}

func List() Metrics {
	collected.mu.Lock()
	defer collected.mu.Unlock()

	var (
		sorted  = make([]string, 0, len(collected.stats))
		longest = 0
	)
	for k := range collected.stats {
		sorted = append(sorted, k)
		if len(k) > longest {
			longest = len(k)
		}
	}
	sort.Strings(sorted)

	x := make(Metrics, 0, len(sorted))

	for _, k := range sorted {
		x = append(x, struct {
			Tag   string
			Times ztime.Durations
		}{Tag: k, Times: collected.stats[k]})
	}
	return x
}

// Metric is a single metric that's being recorded.
type Metric struct {
	tag   string
	start time.Time
}

// Start recording performance metrics with the given tag.
func Start(tag string) *Metric {
	return &Metric{tag: tag, start: time.Now()}
}

// Done finishes recording this performance metrics, and actually records it.
func (t *Metric) Done() {
	collected.add(t.tag, time.Since(t.start))
}

// AddTag adds another part to this metric's tag, for example:
//
//	m := Start("hello")
//	defer m.Done()
//
//	if isCached {
//	    m.AddTag("cached")
//	    return cachedItem
//	}
//
// This will record the cached entries as "hello.cached", separate from the
// regular "hello" entries.
func (t *Metric) AddTag(tag string) {
	t.tag += "." + tag
}

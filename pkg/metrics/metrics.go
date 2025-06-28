// Package metrics collects performance metrics.
package metrics

import (
	"fmt"
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
	}
	t.Append(d)
	m.stats[tag] = t
}

type Metrics []struct {
	Tag   string
	Times ztime.Durations
}

// Sort returns a copy sorted by the given metric.
func (m Metrics) Sort(metric string) Metrics {
	var f func(i, j int) bool
	switch metric {
	case "sum", "total":
		f = func(i, j int) bool { return m[i].Times.Sum() > m[j].Times.Sum() }
	case "mean":
		f = func(i, j int) bool { return m[i].Times.Mean() > m[j].Times.Mean() }
	case "median":
		f = func(i, j int) bool { return m[i].Times.Median() > m[j].Times.Median() }
	case "min":
		f = func(i, j int) bool { return m[i].Times.Min() > m[j].Times.Min() }
	case "max":
		f = func(i, j int) bool { return m[i].Times.Max() > m[j].Times.Max() }
	case "len":
		f = func(i, j int) bool { return m[i].Times.Len() > m[j].Times.Len() }
	default:
		panic(fmt.Sprintf("Metrics.Sort: unknown column: %q", metric))
	}

	sort.Slice(m, f)
	return m
}

// List metrics, sorted by name.
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
	t.tag += "·" + tag
}

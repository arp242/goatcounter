// Copyright © Martin Tournoij – This file is part of GoatCounter and published
// under the terms of a slightly modified EUPL v1.2 license, which can be found
// in the LICENSE file or at https://license.goatcounter.com

package metrics

import (
	"fmt"
	"testing"
	"time"
)

func TestMetrics(t *testing.T) {
	{
		m := Start("test")
		time.Sleep(10 * time.Millisecond)
		m.Done()
	}
	{
		m := Start("test")
		time.Sleep(20 * time.Millisecond)
		m.Done()
	}

	{
		m := Start("test")
		m.AddTag("x")
		time.Sleep(15 * time.Millisecond)
		m.Done()
	}

	tr := func(d time.Duration) time.Duration { return d.Truncate(time.Millisecond) }

	have := ""
	for _, l := range List() {
		have += fmt.Sprintf("%s\t%s\t%s\t%s\n", l.Tag,
			tr(l.Times.Sum()), tr(l.Times.Min()), tr(l.Times.Max()))
	}

	want := `
test	30ms	10ms	20ms
test·x	15ms	15ms	15ms
`[1:]

	if want != have {
		t.Errorf("\nwant:\n%shave:\n%s", want, have)
	}
}

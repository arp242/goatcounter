package metrics

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"
)

func TestMetrics(t *testing.T) {
	if _, ok := os.LookupEnv("CI"); ok {
		// Because the CI is quite slow, it may take more than a millisecond.
		t.Skip("flaky in CI")
	}

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

	var have strings.Builder
	for _, l := range List() {
		have.WriteString(fmt.Sprintf("%s\t%s\t%s\t%s\n", l.Tag,
			tr(l.Times.Sum()), tr(l.Times.Min()), tr(l.Times.Max())))
	}

	want := `
test	30ms	10ms	20ms
test·x	15ms	15ms	15ms
`[1:]

	if want != have.String() {
		t.Errorf("\nwant:\n%shave:\n%s", want, have.String())
	}
}

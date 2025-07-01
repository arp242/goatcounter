package log

import (
	"bytes"
	"log/slog"
	"regexp"
	"testing"

	"zgo.at/zstd/ztest"
)

func TestChain(t *testing.T) {
	buf := new(bytes.Buffer)
	l := slog.New(NewChain(
		slog.NewTextHandler(buf, &slog.HandlerOptions{Level: slog.LevelInfo}),
		slog.NewJSONHandler(buf, &slog.HandlerOptions{Level: slog.LevelWarn}),
	))
	l.Debug("dbg")
	l.Info("information")
	l.WithGroup("g").With("a", "b").Warn("warning")
	l.Error("warning")

	re := regexp.MustCompile(`\d\d\d\d-\d\d-\d\dT\d\d:\d\d:\d\d\.\d{3,9}[+-]\d\d:\d\d`)
	have := re.ReplaceAllString(buf.String(), `2006-01-02T15:04:05.999-07:00`)

	want := `
		time=2006-01-02T15:04:05.999-07:00 level=INFO msg=information
		time=2006-01-02T15:04:05.999-07:00 level=WARN msg=warning g.a=b
		{"time":"2006-01-02T15:04:05.999-07:00","level":"WARN","msg":"warning","g":{"a":"b"}}
		time=2006-01-02T15:04:05.999-07:00 level=ERROR msg=warning
		{"time":"2006-01-02T15:04:05.999-07:00","level":"ERROR","msg":"warning"}
	`
	if d := ztest.Diff(have, want, ztest.DiffNormalizeWhitespace); d != "" {
		t.Error(d)
	}
}

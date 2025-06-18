package log

import (
	"bytes"
	"context"
	"log/slog"
	"net/http"
	"regexp"
	"testing"
	"time"

	"zgo.at/slog_align"
	"zgo.at/zli"
	"zgo.at/zstd/ztest"
)

func TestMain(m *testing.M) {
	zli.WantColor = false
	now = func() time.Time { return time.Date(1985, 6, 18, 13, 14, 15, 123, time.UTC) }
	m.Run()
}

func newAlign(buf *bytes.Buffer) slog.Handler {
	h := slog_align.NewAlignedHandler(buf, &slog.HandlerOptions{
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if a.Key == "module" {
				return slog.Attr{}
			}
			return a
		},
	})
	h.SetColor(false)
	h.SetInlineLocation(true)
	return h
}

func TestContext(t *testing.T) {
	// Replace line numbers from stack trace, so we're not constantly
	// updating the tests.
	re := regexp.MustCompile(`(log|log_test)\.go:\d+`)

	do := func(ctx context.Context) {
		Info(ctx, "MSG", "attr", 123)
		Module("MOD").Info(ctx, "MSG", "attr", 123)
		Error(ctx, "oh noes", "some attr", "xxx")
		Module("M").Error(ctx, "oh noes", "some attr", "xxx")
	}

	doJSON := func(ctx context.Context) string {
		buf := new(bytes.Buffer)
		slog.SetDefault(slog.New(slog.NewJSONHandler(buf, nil)))
		do(ctx)
		return re.ReplaceAllString(buf.String(), `$1.go:XXX`)
	}

	doAlign := func(ctx context.Context) string {
		buf := new(bytes.Buffer)
		slog.SetDefault(slog.New(newAlign(buf)))
		do(ctx)
		return re.ReplaceAllString(buf.String(), `$1.go:XXX`)
	}

	t.Run("empty context", func(t *testing.T) {
		ctx := context.Background()

		t.Run("align", func(t *testing.T) {
			have := doAlign(ctx)
			want := `
				13:14 INFO  MSG  [pkg/log/log.go:XXX]
				attr = 123
				13:14 INFO  MOD: MSG  [pkg/log/log_test.go:XXX]
				attr = 123
				13:14 ERROR oh noes  [pkg/log/log.go:XXX]
				stacktrace =
				log.go:XXX   zgo.at/goatcounter/v2/pkg/log.(*Logger).Error
				some attr  = xxx
				13:14 ERROR M: oh noes  [pkg/log/log_test.go:XXX]
				stacktrace =
				log.go:XXX   zgo.at/goatcounter/v2/pkg/log.(*Logger).Error
				some attr  = xxx
			`
			if d := ztest.Diff(have, want, ztest.DiffNormalizeWhitespace); d != "" {
				t.Error(d)
			}
		})

		t.Run("json", func(t *testing.T) {
			have := doJSON(ctx)
			want := `
				{"time":"1985-06-18T13:14:15.000000123Z","level":"INFO","msg":"MSG","attr":123}
				{"time":"1985-06-18T13:14:15.000000123Z","level":"INFO","msg":"MOD: MSG","attr":123,"module":"MOD"}
				{"time":"1985-06-18T13:14:15.000000123Z","level":"ERROR","msg":"oh noes","stacktrace":"\n\tlog.go:XXX   zgo.at/goatcounter/v2/pkg/log.(*Logger).Error\n","some attr":"xxx"}
				{"time":"1985-06-18T13:14:15.000000123Z","level":"ERROR","msg":"M: oh noes","stacktrace":"\n\tlog.go:XXX   zgo.at/goatcounter/v2/pkg/log.(*Logger).Error\n","some attr":"xxx","module":"M"}
			`
			if d := ztest.Diff(have, want, ztest.DiffNormalizeWhitespace); d != "" {
				t.Error(d)
			}
		})
	})

	t.Run("attrs from context", func(t *testing.T) {
		ctx := WithLog(context.Background(), "one", 1)
		ctx = WithLog(ctx, "moar", []any{"xx", "yy"})
		ctx = WithLog(ctx, "one", "another one")

		t.Run("align", func(t *testing.T) {
			have := doAlign(ctx)
			want := `
				13:14 INFO  MSG  [pkg/log/log.go:XXX]
				attr = 123
				one  = 1
				moar = [xx yy]
				one  = another one
				13:14 INFO  MOD: MSG  [pkg/log/log_test.go:XXX]
				attr = 123
				one  = 1
				moar = [xx yy]
				one  = another one
				13:14 ERROR oh noes  [pkg/log/log.go:XXX]
				stacktrace =
				log.go:XXX   zgo.at/goatcounter/v2/pkg/log.(*Logger).Error
				some attr  = xxx
				one        = 1
				moar       = [xx yy]
				one        = another one
				13:14 ERROR M: oh noes  [pkg/log/log_test.go:XXX]
				stacktrace =
				log.go:XXX   zgo.at/goatcounter/v2/pkg/log.(*Logger).Error
				some attr  = xxx
				one        = 1
				moar       = [xx yy]
				one        = another one
			`
			if d := ztest.Diff(have, want, ztest.DiffNormalizeWhitespace); d != "" {
				t.Error(d)
			}
		})

		t.Run("json", func(t *testing.T) {
			have := doJSON(ctx)
			want := `
				{"time":"1985-06-18T13:14:15.000000123Z","level":"INFO","msg":"MSG","attr":123,"one":1,"moar":["xx","yy"],"one":"another one"}
				{"time":"1985-06-18T13:14:15.000000123Z","level":"INFO","msg":"MOD: MSG","attr":123,"module":"MOD","one":1,"moar":["xx","yy"],"one":"another one"}
				{"time":"1985-06-18T13:14:15.000000123Z","level":"ERROR","msg":"oh noes","stacktrace":"\n\tlog.go:XXX   zgo.at/goatcounter/v2/pkg/log.(*Logger).Error\n","some attr":"xxx","one":1,"moar":["xx","yy"],"one":"another one"}
				{"time":"1985-06-18T13:14:15.000000123Z","level":"ERROR","msg":"M: oh noes","stacktrace":"\n\tlog.go:XXX   zgo.at/goatcounter/v2/pkg/log.(*Logger).Error\n","some attr":"xxx","module":"M","one":1,"moar":["xx","yy"],"one":"another one"}
			`
			if d := ztest.Diff(have, want, ztest.DiffNormalizeWhitespace); d != "" {
				t.Error(d)
			}
		})
	})
}

func TestDebug(t *testing.T) {
	buf := new(bytes.Buffer)
	slog.SetDefault(slog.New(slog.NewJSONHandler(buf, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})))
	SetDebug([]string{"logme"})
	ctx := context.Background()

	Debug(ctx, "should not be logged")
	Module("nope").Debug(ctx, "should not be logged")
	Module("logme").Debug(ctx, "captain's log")

	have := buf.String()
	want := `
		{"time":"1985-06-18T13:14:15.000000123Z","level":"DEBUG","msg":"logme: captain's log","module":"logme"}
	`
	if d := ztest.Diff(have, want, ztest.DiffNormalizeWhitespace); d != "" {
		t.Error(d)
	}
}

func TestAttrHTTP(t *testing.T) {
	r, _ := http.NewRequest("GET", "http://example.com/path", nil)
	r.Header.Set("User-Agent", "Mozilla/5.0 TestBrowser/1.0")

	t.Run("align", func(t *testing.T) {
		buf := new(bytes.Buffer)
		slog.SetDefault(slog.New(newAlign(buf)))

		Info(context.Background(), "msg", "attr1", "val1", AttrHTTP(r))
		have := buf.String()
		want := `
			13:14 INFO  msg  [pkg/log/log.go:106]
			             attr1     = val1
			             http.verb = GET
			             http.url  = http://example.com/path
			             http.host = example.com
			             http.ua   = ~Z TestBrowser/1.0
			`
		if d := ztest.Diff(have, want, ztest.DiffNormalizeWhitespace); d != "" {
			t.Error(d)
		}
	})

	t.Run("json", func(t *testing.T) {
		buf := new(bytes.Buffer)
		slog.SetDefault(slog.New(slog.NewJSONHandler(buf, nil)))

		Info(context.Background(), "msg", "attr1", "val1", AttrHTTP(r))
		have := buf.String()
		want := `{
			"attr1": "val1",
			"http": {
				"host": "example.com",
				"verb": "GET",
				"url": "http://example.com/path",
				"ua": "~Z TestBrowser/1.0"
			},
			"level": "INFO",
			"msg": "msg",
			"time": "1985-06-18T13:14:15.000000123Z"
		}`
		if d := ztest.Diff(have, want, ztest.DiffJSON); d != "" {
			t.Error(d)
		}
	})
}

func TestRecover(t *testing.T) {
	buf := new(bytes.Buffer)
	slog.SetDefault(slog.New(newAlign(buf)))
	go func() {
		defer Recover(context.Background())
		panic("oh noes!")
	}()
	time.Sleep(20 * time.Millisecond)
	//fmt.Println(buf.String())
}

// Package log wraps slog.
//
// This is mostly to allow enabling/disabling logs per module:
//
//	l := slog.Module("module-name")
//	l.Info("msg")
//	l.Error("msg")
//	l.Debug("msg")
//
// And the Debug() calls are hidden by default, and show up if it's enabled with
// a -debug flag. I can set the level in slog, but only globally.
package log

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"runtime"
	"runtime/debug"
	"slices"
	"sync"
	"time"

	"zgo.at/errors"
	"zgo.at/gadget"
	"zgo.at/zstd/zdebug"
)

var ctxkey = &struct{ n string }{"log"}

// WithLog returns a context with log attributes.
//
// Previous attributes are kept, so something like this:
//
//	ctx = log.WithLog(ctx, "request", rnd())
//	ctx = log.WithLog(ctx, "user_id", u.ID)
//
// Will result in both the request and user_id attributes, rather than user_id
// overwriting it.
//
// Note that it doesn't check for duplicates; so:
//
//	ctx = log.WithLog(ctx, "attr", 1)
//	ctx = log.WithLog(ctx, "attr", 2)
//
// Will result in two "attr" attributes: one with the value "1" and one with
// "2".
func WithLog(ctx context.Context, attrs ...any) context.Context {
	exist := Get(ctx)
	return context.WithValue(ctx, ctxkey, append(exist, attrs...))
}

// Get attributes from context.
func Get(ctx context.Context) []any {
	a, ok := ctx.Value(ctxkey).([]any)
	if !ok {
		return nil
	}
	return a
}

// OnError is a hook called for every eror log.
var OnError func(module string, r slog.Record)

func strOrErr(msg any) (string, []any) {
	switch m := msg.(type) {
	default:
		panic(fmt.Sprintf("log.Error: msg must be string or error, not %T", m))
	case string:
		return m, nil
	case error:
		var (
			attr = []any{"_err", m}
			sErr = new(errors.StackErr)
		)
		if !errors.As(m, &sErr) {
			return m.Error(), attr
		}
		if t := sErr.StackTrace(); t != "" {
			attr = append(attr, "stacktrace", "\n"+t)
		}
		return sErr.Unwrap().Error(), attr
	}
}

var (
	doDebug   []string
	doDebugMu sync.RWMutex
)

func SetDebug(l []string) { doDebugMu.Lock(); defer doDebugMu.Unlock(); doDebug = l }
func ListDebug() []string { doDebugMu.RLock(); defer doDebugMu.RUnlock(); return doDebug }
func HasDebug(module string) bool {
	doDebugMu.RLock()
	defer doDebugMu.RUnlock()
	return !slices.Contains(doDebug, "-"+module) &&
		(slices.Contains(doDebug, module) || slices.Contains(doDebug, "all"))
}

func Module(module string) *Logger                           { return &Logger{module: module} }
func With(args ...any) *Logger                               { return Module("").With(args...) }
func Error(ctx context.Context, msg any, attr ...any)        { Module("").Error(ctx, msg, attr...) }
func Warn(ctx context.Context, msg string, attr ...any)      { Module("").Warn(ctx, msg, attr...) }
func Info(ctx context.Context, msg string, attr ...any)      { Module("").Info(ctx, msg, attr...) }
func Debug(ctx context.Context, msg string, attr ...any)     { Module("").Debug(ctx, msg, attr...) }
func Errorf(ctx context.Context, format string, args ...any) { Module("").Errorf(ctx, format, args...) }
func Warnf(ctx context.Context, format string, args ...any)  { Module("").Warnf(ctx, format, args...) }
func Infof(ctx context.Context, format string, args ...any)  { Module("").Infof(ctx, format, args...) }
func Debugf(ctx context.Context, format string, args ...any) { Module("").Debugf(ctx, format, args...) }

type Logger struct {
	module string
	attr   []any
}

func (l *Logger) With(args ...any) *Logger { l.attr = append(l.attr, args...); return l }

func (l *Logger) Error(ctx context.Context, msg any, attr ...any) {
	logger := slog.Default()
	if !logger.Enabled(context.Background(), slog.LevelError) {
		return
	}
	logmsg, more := strOrErr(msg)
	attr = append(attr, more...)
	r := l.newRecord(ctx, slog.LevelError, logmsg, attr...)
	if OnError != nil {
		OnError(l.module, r)
	}
	err := logger.Handler().Handle(context.Background(), r)
	if err != nil {
		fmt.Fprintf(os.Stderr, "logger.Handle error: %s\n", err)
	}
}
func (l *Logger) Warn(ctx context.Context, msg string, attr ...any) {
	logger := slog.Default()
	if !logger.Enabled(context.Background(), slog.LevelWarn) {
		return
	}
	err := logger.Handler().Handle(context.Background(), l.newRecord(ctx, slog.LevelWarn, msg, attr...))
	if err != nil {
		fmt.Fprintf(os.Stderr, "logger.Handle error: %s\n", err)
	}
}
func (l *Logger) Info(ctx context.Context, msg string, attr ...any) {
	logger := slog.Default()
	if !logger.Enabled(context.Background(), slog.LevelInfo) {
		return
	}
	err := logger.Handler().Handle(context.Background(), l.newRecord(ctx, slog.LevelInfo, msg, attr...))
	if err != nil {
		fmt.Fprintf(os.Stderr, "logger.Handle error: %s\n", err)
	}
}
func (l *Logger) Debug(ctx context.Context, msg string, attr ...any) {
	logger := slog.Default()
	if !HasDebug(l.module) {
		return
	}
	err := logger.Handler().Handle(context.Background(), l.newRecord(ctx, slog.LevelDebug, msg, attr...))
	if err != nil {
		fmt.Fprintf(os.Stderr, "logger.Handle error: %s\n", err)
	}
}

func (l *Logger) Errorf(ctx context.Context, format string, args ...any) {
	logger := slog.Default()
	if !logger.Enabled(context.Background(), slog.LevelError) {
		return
	}
	r := l.newRecord(ctx, slog.LevelError, fmt.Sprintf(format, args...))
	if OnError != nil {
		OnError(l.module, r)
	}
	err := logger.Handler().Handle(context.Background(), r)
	if err != nil {
		fmt.Fprintf(os.Stderr, "logger.Handle error: %s\n", err)
	}
}
func (l *Logger) Warnf(ctx context.Context, format string, args ...any) {
	logger := slog.Default()
	if !logger.Enabled(context.Background(), slog.LevelWarn) {
		return
	}
	err := logger.Handler().Handle(context.Background(), l.newRecord(ctx, slog.LevelWarn, fmt.Sprintf(format, args...)))
	if err != nil {
		fmt.Fprintf(os.Stderr, "logger.Handle error: %s\n", err)
	}
}
func (l *Logger) Infof(ctx context.Context, format string, args ...any) {
	logger := slog.Default()
	if !logger.Enabled(context.Background(), slog.LevelInfo) {
		return
	}
	err := logger.Handler().Handle(context.Background(), l.newRecord(ctx, slog.LevelInfo, fmt.Sprintf(format, args...)))
	if err != nil {
		fmt.Fprintf(os.Stderr, "logger.Handle error: %s\n", err)
	}
}
func (l *Logger) Debugf(ctx context.Context, format string, args ...any) {
	logger := slog.Default()
	if !HasDebug(l.module) {
		return
	}
	err := logger.Handler().Handle(context.Background(), l.newRecord(ctx, slog.LevelDebug, fmt.Sprintf(format, args...)))
	if err != nil {
		fmt.Fprintf(os.Stderr, "logger.Handle error: %s\n", err)
	}
}

var now = func() time.Time { return time.Now().UTC() }

func (l *Logger) newRecord(ctx context.Context, level slog.Level, msg string, attr ...any) slog.Record {
	if l.module != "" {
		msg = l.module + ": " + msg
	}

	var pcs [1]uintptr
	runtime.Callers(3, pcs[:]) // skip [Callers, {Error,Info,...}, newRecord]
	r := slog.NewRecord(now(), level, msg, pcs[0])
	if level == slog.LevelError {
		r.Add("stacktrace", "\n"+string(zdebug.Stack(
			"github.com/go-chi/chi/v5.(*ChainHandler).ServeHTTP",
			"github.com/go-chi/chi/v5.(*Mux).ServeHTTP",
			"github.com/go-chi/chi/v5.(*Mux).routeHTTP",
			"github.com/go-chi/chi/v5/middleware",
			"golang.org/x/net/http2/h2c.h2cHandler.ServeHTTP",
			"net/http.(*conn).serve",
			"net/http.HandlerFunc.ServeHTTP",
			"net/http.serverHandler.ServeHTTP",
			"zgo.at/goatcounter/v2/handlers.backend.Mount.Delay",
			"zgo.at/goatcounter/v2/handlers.backend.Mount.NoStore",
			"zgo.at/goatcounter/v2/handlers.backend.Mount.RealIP",
			"zgo.at/goatcounter/v2/handlers.backend.Mount.Unpanic",
			"zgo.at/goatcounter/v2/handlers.backend.Mount.Wrap",
			"zgo.at/goatcounter/v2/handlers.backend.Mount.WrapWriter",
			"zgo.at/goatcounter/v2/handlers.backend.Mount.addcsp",
			"zgo.at/goatcounter/v2/handlers.backend.Mount.addctx",
			"zgo.at/goatcounter/v2/handlers.backend.Mount.addz18n",
			"zgo.at/goatcounter/v2/handlers.init.Add",
			"zgo.at/goatcounter/v2/handlers.init.Filter",
			"zgo.at/goatcounter/v2/pkg/log.(*Logger).newRecord",
			"zgo.at/goatcounter/v2/pkg/log.Error",
			"zgo.at/goatcounter/v2/pkg/log.Test",
			"zgo.at/zhttp.HostRoute",
			"zgo.at/zhttp/mware",
		)))
	}
	r.Add(l.attr...)
	r.Add(attr...)
	if l.module != "" { // Removed with ReplaceAttr when creating slog_align handler.
		r.Add("module", l.module)
	}
	if c := Get(ctx); len(c) > 0 {
		r.Add(c...)
	}
	return r
}

// AttrHTTP adds attributes from a HTTP request.
func AttrHTTP(r *http.Request) slog.Attr {
	return slog.Group("http",
		"verb", r.Method,
		"url", r.URL.String(),
		"host", r.Host,
		"ua", gadget.ShortenUA(r.UserAgent()),
	)
}

// Recover from a panic.
//
// Any panics will be recover()'d and reported with Error():
//
//	go func() {
//	    defer log.Recover(ctx)
//	    // ... do work...
//	}()
//
// You can optionally specify your own error log in the callback, for example if
// you want to add some attributes:
//
//	defer log.Recover(ctx, func(err error) {
//	    log.Error(err, "field", value)
//	})
func Recover(ctx context.Context, cb ...func(error)) {
	r := recover()
	if r == nil {
		return
	}

	err, ok := r.(error)
	if !ok {
		err = fmt.Errorf("%v", r)
	}
	err = fmt.Errorf("%w\n%s", err, debug.Stack())

	if len(cb) > 0 && cb[0] != nil {
		cb[0](err)
	} else {
		Module("panic").Error(ctx, err)
	}
}

package log

import (
	"context"
	"errors"
	"log/slog"
	"slices"
)

type Chain struct {
	handlers []slog.Handler
}

func NewChain(handlers ...slog.Handler) Chain {
	return Chain{handlers}
}

func (h Chain) Enabled(ctx context.Context, l slog.Level) bool {
	for _, hh := range h.handlers {
		if hh.Enabled(ctx, l) {
			return true
		}
	}
	return false
}
func (h Chain) WithAttrs(attrs []slog.Attr) slog.Handler {
	cp := make([]slog.Handler, 0, len(h.handlers))
	for _, hh := range h.handlers {
		cp = append(cp, hh.WithAttrs(slices.Clone(attrs)))
	}
	return Chain{cp}
}
func (h Chain) WithGroup(name string) slog.Handler {
	if name == "" {
		return h
	}
	cp := make([]slog.Handler, 0, len(h.handlers))
	for _, hh := range h.handlers {
		cp = append(cp, hh.WithGroup(name))
	}
	return Chain{cp}
}
func (h Chain) Handle(ctx context.Context, r slog.Record) error {
	var errs []error
	for _, hh := range h.handlers {
		if hh.Enabled(ctx, r.Level) {
			err := hh.Handle(ctx, r)
			if err != nil {
				errs = append(errs, err)
			}
		}
	}
	return errors.Join(errs...)
}

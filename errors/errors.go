// Copyright © 2019 Martin Tournoij <martin@arp242.net>
// This file is part of GoatCounter and published under the terms of the EUPL
// v1.2, which can be found in the LICENSE file or at http://eupl12.zgo.at

// Package errors adds Wrap(), Wrapf(), and stack traces to stdlib's errors.
//
// Wrap() removes the need for quite a few if err != nil checks and makes
// migrating from pkg/errors to Go 1.13 errors a bit easier.
package errors

import (
	"errors"
	"fmt"
	"runtime"
	"strings"
)

func New(text string) error                 { return addStack(errors.New(text)) }
func Unwrap(err error) error                { return addStack(errors.Unwrap(err)) }
func Is(err, target error) bool             { return errors.Is(err, target) }
func As(err error, target interface{}) bool { return errors.As(err, target) }

// Wrap an error with fmt.Errorf(), returning nil if err is nil.
func Wrap(err error, s string) error {
	if err == nil {
		return nil
	}
	return addStack(fmt.Errorf(s+": %w", err))
}

// Wrapf an error with fmt.Errorf(), returning nil if err is nil.
func Wrapf(err error, format string, a ...interface{}) error {
	if err == nil {
		return nil
	}
	return addStack(fmt.Errorf(format+": %w", append(a, err)...))
}

// Errorf creates a new error. This is like fmt.Errorf() but also adds a stack
// trace.
func Errorf(format string, a ...interface{}) error {
	return addStack(fmt.Errorf(format, a...))
}

func addStack(err error) error {
	pc := make([]uintptr, 32)
	n := runtime.Callers(3, pc)
	pc = pc[:n]

	frames := runtime.CallersFrames(pc)

	var b strings.Builder
	for {
		frame, more := frames.Next()
		if !strings.HasPrefix(frame.Function, "zgo.at/goatcounter") {
			if !more {
				break
			}
			continue
		}

		// Don't format exactly the same as debug.PrintStack(); memory addresses
		// aren't very useful here and only add to the noise.
		b.WriteString(fmt.Sprintf("\t%s()\n\t\t%s:%d\n", frame.Function, frame.File, frame.Line))
		if !more {
			break
		}
	}

	return &stackErr{err: err, stack: b.String()}
}

type stackErr struct {
	stack string
	err   error
}

func (err stackErr) Unwrap() error { return err.err }

func (err stackErr) Error() string {
	if err.stack == "" {
		return fmt.Sprintf("%s", err.err)
	}
	return fmt.Sprintf("%s\n%s", err.err, err.stack)
}

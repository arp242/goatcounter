// Copyright © 2019 Martin Tournoij <martin@arp242.net>
// This file is part of GoatCounter and published under the terms of the EUPL
// v1.2, which can be found in the LICENSE file or at http://eupl12.zgo.at

// Package errors adds Wrap() and Wrapf() to stdlib's errors.
//
// This removes the need for quite a few if err != nil checks and makes
// migrating from pkg/errors to Go 1.13 errors a bit easier.
package errors

import (
	"errors"
	"fmt"
)

func New(text string) error                 { return errors.New(text) }
func Unwrap(err error) error                { return errors.Unwrap(err) }
func Is(err, target error) bool             { return errors.Is(err, target) }
func As(err error, target interface{}) bool { return errors.As(err, target) }

// Wrap an error with fmt.Errorf(), returning nil if err is nil.
func Wrap(err error, s string) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf(s+": %w", err)
}

// Wrapf an error with fmt.Errorf(), returning nil if err is nil.
func Wrapf(err error, format string, a ...interface{}) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf(format+": %w", append(a, err)...)
}

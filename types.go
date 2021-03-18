// Copyright © 2019 Martin Tournoij – This file is part of GoatCounter and
// published under the terms of a slightly modified EUPL v1.2 license, which can
// be found in the LICENSE file or at https://license.goatcounter.com

package goatcounter

import (
	"database/sql/driver"
	"fmt"
	"strings"

	"zgo.at/zstd/zfloat"
	"zgo.at/zstd/zint"
	"zgo.at/zstd/zstring"
)

// Ints stores a slice of []int64 as a comma-separated string.
type Ints []int64

func (l Ints) String() string                { return zint.Join(l, ", ") }
func (l Ints) Value() (driver.Value, error)  { return zint.Join(l, ","), nil }
func (l *Ints) UnmarshalText(v []byte) error { return l.Scan(v) }

func (l *Ints) Scan(v interface{}) error {
	if v == nil {
		return nil
	}

	var err error
	*l, err = zint.Split(fmt.Sprintf("%s", v), ",")
	return err
}

func (l Ints) MarshalText() ([]byte, error) {
	v, err := l.Value()
	return []byte(fmt.Sprintf("%s", v)), err
}

// Floats stores a slice of []float64 as a comma-separated string.
type Floats []float64

func (l Floats) String() string                { return zfloat.Join(l, ", ") }
func (l Floats) Value() (driver.Value, error)  { return zfloat.Join(l, ","), nil }
func (l *Floats) UnmarshalText(v []byte) error { return l.Scan(v) }

func (l *Floats) Scan(v interface{}) error {
	if v == nil {
		return nil
	}

	var err error
	*l, err = zfloat.Split(fmt.Sprintf("%s", v), ",")
	return err
}

func (l Floats) MarshalText() ([]byte, error) {
	v, err := l.Value()
	return []byte(fmt.Sprintf("%s", v)), err
}

// Strings stores a slice of []string as a comma-separated string.
type Strings []string

func (l Strings) String() string { return strings.Join(l, ", ") }
func (l Strings) Value() (driver.Value, error) {
	return strings.Join(zstring.Filter(l, zstring.FilterEmpty), ","), nil
}
func (l *Strings) UnmarshalText(v []byte) error { return l.Scan(v) }

// TODO: move to zstd/zstring
func splitAny(s string, seps ...string) []string {
	var split []string
	for {
		var i int
		for _, sep := range seps {
			i = strings.Index(s, sep)
			if i >= 0 {
				break
			}
		}
		if i < 0 {
			if len(s) > 0 {
				split = append(split, s)
			}
			break
		}
		split = append(split, s[:i])
		s = s[i+1:]
	}

	return split
}

func (l *Strings) Scan(v interface{}) error {
	if v == nil {
		return nil
	}

	split := splitAny(fmt.Sprintf("%s", v), ",", " ")
	strs := make([]string, 0, len(split))
	for _, s := range split {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		strs = append(strs, s)
	}
	*l = strs
	return nil
}

func (l Strings) MarshalText() ([]byte, error) {
	v, err := l.Value()
	return []byte(fmt.Sprintf("%s", v)), err
}

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

func (l Ints) String() string {
	return zint.Join(l, ", ")
}

// Value determines what to store in the DB.
func (l Ints) Value() (driver.Value, error) {
	return zint.Join(l, ","), nil
}

// Scan converts the data from the DB.
func (l *Ints) Scan(v interface{}) error {
	if v == nil {
		return nil
	}

	var err error
	*l, err = zint.Split(fmt.Sprintf("%s", v), ",")
	return err
}

// MarshalText converts the data to a human readable representation.
func (l Ints) MarshalText() ([]byte, error) {
	v, err := l.Value()
	return []byte(fmt.Sprintf("%s", v)), err
}

// UnmarshalText parses text in to the Go data structure.
func (l *Ints) UnmarshalText(v []byte) error {
	return l.Scan(v)
}

// Floats stores a slice of []float64 as a comma-separated string.
type Floats []float64

func (l Floats) String() string {
	return zfloat.Join(l, ", ")
}

// Value determines what to store in the DB.
func (l Floats) Value() (driver.Value, error) {
	return zfloat.Join(l, ","), nil
}

// Scan converts the data from the DB.
func (l *Floats) Scan(v interface{}) error {
	if v == nil {
		return nil
	}

	var err error
	*l, err = zfloat.Split(fmt.Sprintf("%s", v), ",")
	return err
}

// MarshalText converts the data to a human readable representation.
func (l Floats) MarshalText() ([]byte, error) {
	v, err := l.Value()
	return []byte(fmt.Sprintf("%s", v)), err
}

// UnmarshalText parses text in to the Go data structure.
func (l *Floats) UnmarshalText(v []byte) error {
	return l.Scan(v)
}

// Strings stores a slice of []string as a comma-separated string.
//
// Note this only works for simple strings (e.g. enums), it DOES NOT ESCAPE
// COMMAS, and you will run in to problems if you use it for arbitrary text.
//
// You're probably better off using e.g. arrays in PostgreSQL or JSON in SQLite,
// if you can. This is intended just for simple cross-SQL-engine use cases.
type Strings []string

func (l Strings) String() string {
	return strings.Join(l, ", ")
}

// Value determines what to store in the DB.
func (l Strings) Value() (driver.Value, error) {
	return strings.Join(zstring.Filter(l, zstring.FilterEmpty), ","), nil
}

// Scan converts the data from the DB.
func (l *Strings) Scan(v interface{}) error {
	if v == nil {
		return nil
	}
	strs := []string{}
	for _, s := range strings.Split(fmt.Sprintf("%s", v), ",") {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		strs = append(strs, s)
	}
	*l = strs
	return nil
}

// MarshalText converts the data to a human readable representation.
func (l Strings) MarshalText() ([]byte, error) {
	v, err := l.Value()
	return []byte(fmt.Sprintf("%s", v)), err
}

// UnmarshalText parses text in to the Go data structure.
func (l *Strings) UnmarshalText(v []byte) error {
	return l.Scan(v)
}

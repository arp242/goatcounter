// Copyright © Martin Tournoij – This file is part of GoatCounter and published
// under the terms of a slightly modified EUPL v1.2 license, which can be found
// in the LICENSE file or at https://license.goatcounter.com

//go:generate go run gen.go

package goatcounter

import (
	"context"
	"embed"
	"fmt"
	"math"
	"strconv"

	"github.com/google/uuid"
	"github.com/mattn/go-sqlite3"
	"zgo.at/z18n"
	"zgo.at/zdb"
	"zgo.at/zstd/zcrypto"
	"zgo.at/zstd/zint"
	"zgo.at/zvalidate"
)

// DB contains all files in db/*
//
//go:embed db/schema.gotxt
//go:embed db/languages.sql
//go:embed db/migrate/*.sql
//go:embed db/migrate/*.gotxt
//go:embed db/query/*
var DB embed.FS

// Static contains all the static files to serve.
//
//go:embed public/*
var Static embed.FS

// Templates contains all templates.
//
//go:embed tpl/*
var Templates embed.FS

// GeoDB contains the GeoIP countries database.
//
//go:embed pack/GeoLite2-Country.mmdb.gz
var GeoDB []byte

// State column values.
const (
	StateActive  = "a"
	StateRequest = "r"
	StateDeleted = "d"
)

var States = []string{StateActive, StateRequest, StateDeleted}

var SQLiteHook = func(c *sqlite3.SQLiteConn) error {
	return c.RegisterFunc("percent_diff", func(start, final int) float64 {
		if start == 0 {
			return math.Inf(0)
		}
		return (float64(final - start)) / float64(start) * 100.0
	}, true)
}

// TODO: Move to zdb
func Interval(ctx context.Context, days int) string {
	if zdb.SQLDialect(ctx) == zdb.DialectPostgreSQL {
		return fmt.Sprintf(" now() - interval '%d days' ", days)
	}
	return fmt.Sprintf(" datetime(datetime(), '-%d days') ", days)
}

const numChars = 12

// Compress all the data in to 12 chunks.
func ChunkStat(stats []HitListStat) (int, []int) {
	var (
		chunked   = make([]int, numChars)
		chunkSize = len(stats) * 24 / numChars
		max       = 0
		chunk     = 0
		i         = 0
		n         = 0
	)
	for _, stat := range stats {
		for _, h := range stat.Hourly {
			i++
			chunk += h
			if i == chunkSize {
				chunked[n] = chunk
				if chunk > max {
					max = chunk
				}
				n++
				chunk, i = 0, 0
			}
		}
	}

	return max, chunked
}

func NewBufferKey(ctx context.Context) (string, error) {
	secret := zcrypto.Secret256()
	err := zdb.TX(ctx, func(ctx context.Context) error {
		err := zdb.Exec(ctx, `delete from store where key='buffer-secret'`, nil)
		if err != nil {
			return err
		}

		err = zdb.Exec(ctx, `insert into store (key, value) values ('buffer-secret', :s)`, map[string]any{"s": secret})
		return err
	})
	if err != nil {
		return "", fmt.Errorf("NewBufferKey: %w", err)
	}
	return secret, nil
}

func LoadBufferKey(ctx context.Context) ([]byte, error) {
	var key []byte
	err := zdb.Get(ctx, &key, `select value from store where key='buffer-secret'`)
	if err != nil {
		return nil, fmt.Errorf("LoadBufferKey: %w", err)
	}
	return key, nil
}

// UUID created a new UUID v4.
func UUID() zint.Uint128 {
	u, err := uuid.NewRandom()
	if err != nil {
		panic(fmt.Sprintf("uuid.NewRandom: %s", err))
	}
	i, err := zint.NewUint128(u[:])
	if err != nil {
		panic(err)
	}

	return i
}

func splitIntStr(ident []string) ([]int64, []string) {
	var (
		ids  []int64
		strs []string
	)
	for _, i := range ident {
		id, err := strconv.ParseInt(i, 10, 64)
		if err == nil {
			ids = append(ids, id)
		} else {
			strs = append(strs, i)
		}
	}
	return ids, strs
}

func NewValidate(ctx context.Context) zvalidate.Validator {
	v := zvalidate.New()
	v.Messages(zvalidate.Messages{
		Required:    func() string { return z18n.T(ctx, "validate/required|must be set") },
		Domain:      func() string { return z18n.T(ctx, "validate/domain|must be a valid domain") },
		Hostname:    func() string { return z18n.T(ctx, "validate/hostname|must be a valid hostname") },
		URL:         func() string { return z18n.T(ctx, "validate/url|must be a valid url") },
		Email:       func() string { return z18n.T(ctx, "validate/email|must be a valid email address") },
		IPv4:        func() string { return z18n.T(ctx, "validate/ipv4|must be a valid IPv4 address") },
		IP:          func() string { return z18n.T(ctx, "validate/ip|must be a valid IPv4 or IPv6 address") },
		HexColor:    func() string { return z18n.T(ctx, "validate/color|must be a valid color code") },
		LenLonger:   func() string { return z18n.T(ctx, "validate/len-longer|must be longer than %d characters") },
		LenShorter:  func() string { return z18n.T(ctx, "validate/len-shorter|must be shorter than %d characters") },
		Exclude:     func() string { return z18n.T(ctx, "validate/exclude|cannot be ‘%s’") },
		Include:     func() string { return z18n.T(ctx, "validate/include|must be one of ‘%s’") },
		Integer:     func() string { return z18n.T(ctx, "validate/int|must be a whole number") },
		Bool:        func() string { return z18n.T(ctx, "validate/bool|must be a boolean") },
		Date:        func() string { return z18n.T(ctx, "validate/date|must be a date as ‘%s’") },
		Phone:       func() string { return z18n.T(ctx, "validate/phone|must be a valid phone number") },
		RangeHigher: func() string { return z18n.T(ctx, "validate/range-higher|must be %d or higher") },
		RangeLower:  func() string { return z18n.T(ctx, "validate/range-lower|must be %d or lower") },
		UTF8:        func() string { return z18n.T(ctx, "validate/utf8|must be UTF-8") },
		Contains:    func() string { return z18n.T(ctx, "validate/contains|cannot contain the characters %s") },
	})
	return v
}

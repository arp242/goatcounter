// Copyright © 2019 Martin Tournoij <martin@arp242.net>
// This file is part of GoatCounter and published under the terms of the AGPLv3,
// which can be found in the LICENSE file or at gnu.org/licenses/agpl.html

//go:generate go run gen.go

package goatcounter

import (
	"context"
	"database/sql"
	"fmt"
	"html/template"
	"math"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
	"github.com/mattn/go-sqlite3"
	"zgo.at/zhttp"
	"zgo.at/zhttp/ctxkey"
)

func init() {
	zhttp.FuncMap["validate"] = func(k string, v map[string][]string) template.HTML {
		if v == nil {
			return template.HTML("")
		}
		e, ok := v[k]
		if !ok {
			return template.HTML("")
		}
		return template.HTML(fmt.Sprintf(`<span class="err">Error: %s</span>`,
			template.HTMLEscapeString(strings.Join(e, ", "))))
	}

	// Implemented as function for performance.
	zhttp.FuncMap["bar_chart"] = func(stats []HitStat, max int) template.HTML {
		var b strings.Builder
		for _, stat := range stats {
			for _, s := range stat.Days {
				h := math.Round(float64(s[1]) / float64(max) / 0.01)

				// Double div so that the title is on the entire column, instead
				// of just the coloured area.
				// No need to add the inner one if there's no data – saves quite
				// a bit in the total filesize.
				inner := ""
				if h > 0 {
					inner = fmt.Sprintf(`<div style="height: %.0f%%;"></div>`, h)
				}
				b.WriteString(fmt.Sprintf(`<div title="%s %[2]d:00 – %[2]d:59, %s views">%s</div>`,
					stat.Day, s[0], zhttp.Tnformat(s[1]), inner))
			}
		}

		return template.HTML(b.String())
	}
}

// State column values.
const (
	StateActive  = "a"
	StateRequest = "r"
	StateDeleted = "d"
)

var States = []string{StateActive, StateRequest, StateDeleted}

var _ DB = &sqlx.DB{}
var _ DB = &sqlx.Tx{}

// DB wraps sqlx.DB so we can add transactions and logging.
type DB interface {
	ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error)
	GetContext(ctx context.Context, dest interface{}, query string, args ...interface{}) error
	Rebind(query string) string
	SelectContext(ctx context.Context, dest interface{}, query string, args ...interface{}) error

	//BeginTxx(ctx context.Context, opts *sql.TxOptions) (*sqlx.Tx, error)

	// Rollback() error
	// Commit() error
}

// MustGetDB gets the DB from the context, panicking if this fails.
func MustGetDB(ctx context.Context) DB {
	db, ok := ctx.Value(ctxkey.DB).(DB)
	if !ok {
		panic("MustGetDB: no dbKey value")
	}
	return db
}

// Begin a new transaction.
func Begin(ctx context.Context) (context.Context, *sqlx.Tx, error) {
	tx, err := MustGetDB(ctx).(*sqlx.DB).BeginTxx(ctx, nil)
	return context.WithValue(ctx, ctxkey.DB, tx), tx, err
}

// GetSite gets the current site.
func GetSite(ctx context.Context) *Site {
	s, _ := ctx.Value(ctxkey.Site).(*Site)
	return s
}

// MustGetSite behaves as GetSite(), panicking if this fails.
func MustGetSite(ctx context.Context) *Site {
	s, ok := ctx.Value(ctxkey.Site).(*Site)
	if !ok {
		panic("MustGetSite: no site on context")
	}
	return s
}

// GetUser gets the currently logged in user.
func GetUser(ctx context.Context) *User {
	u, _ := ctx.Value(ctxkey.User).(*User)
	return u
}

func uniqueErr(err error) bool {
	if sqlErr, ok := err.(sqlite3.Error); ok && sqlErr.ExtendedCode == sqlite3.ErrConstraintUnique {
		return true
	}
	if pqErr, ok := err.(pq.Error); ok && pqErr.Code == "23505" {
		return true
	}
	return false
}

func sqlDate(t time.Time) string  { return t.Format("2006-01-02 15:04:05") }
func dayStart(t time.Time) string { return t.Format("2006-01-02") + " 00:00:00" }
func dayEnd(t time.Time) string   { return t.Format("2006-01-02") + " 23:59:59" }

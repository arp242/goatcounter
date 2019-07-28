//go:generate go run gen.go

package goatcounter

import (
	"context"
	"fmt"
	"html/template"
	"strings"

	"github.com/jmoiron/sqlx"
	"github.com/mattn/go-sqlite3"
	"zgo.at/zhttp"
	"zgo.at/zhttp/ctxkey"
)

func init() {
	// Implemented as function for performance.
	//
	// TODO(v1): we can pre-generate this per day; then we just have to fetch the
	// HTML.
	zhttp.FuncMap["bar_chart"] = func(stats []HitStat, max int) template.HTML {
		var b strings.Builder
		for _, stat := range stats {
			for _, s := range stat.Days {
				// Double div so that the title is on the entire column, instead
				// of just the coloured area.
				b.WriteString(fmt.Sprintf(`<div title="%[1]s %[2]d:00 – %[2]d:59, %[3]d views">`+
					`<div style="height: %.2[4]f%%;"></div></div>`,
					stat.Day, s[0], s[1], float64(s[1])/float64(max)/0.01))
			}
		}

		return template.HTML(b.String())
	}

	zhttp.FuncMap["line_chart"] = func(stats []HitStat, max int) template.HTML {
		var b strings.Builder
		b.WriteString("0,50 ") // so fill works as expected.
		c := 1
		for _, stat := range stats {
			for _, s := range stat.Days {
				heightPercentage := float64(s[1]) / float64(max) / 0.01
				heightPixels := heightPercentage / 2
				// TODO: print 50 and 40.1 instead of 50.00 and 40.10 (saves bytes)
				b.WriteString(fmt.Sprintf("%d,%.2f ", c, 50-heightPixels))
				c += 5
			}
		}

		b.WriteString(fmt.Sprintf("%d,50 ", c)) // so fill works as expected.
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

// MustGetDB gets the DB from the context, panicking if this fails.
func MustGetDB(ctx context.Context) *sqlx.DB {
	db, ok := ctx.Value(ctxkey.DB).(*sqlx.DB)
	if !ok {
		panic("MustGetDB: no dbKey value")
	}
	return db
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
	sqlErr, ok := err.(sqlite3.Error)
	return ok && sqlErr.ExtendedCode == sqlite3.ErrConstraintUnique
}

func insert(cols ...string) string {
	c := strings.Join(cols, ",")

	for i := range cols {
		cols[i] = ":" + cols[i]
	}
	v := strings.Join(cols, ",")

	return fmt.Sprintf(" (%s) values (%s) ", c, v)
}

func update(cols ...string) string {
	for i := range cols {
		cols[i] = fmt.Sprintf("%s=:%[1]s", cols[i])
	}
	return " set " + strings.Join(cols, ",") + " "
}

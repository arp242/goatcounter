//go:generate go run gen.go

package goatcounter

import (
	"context"
	"fmt"
	"html/template"
	"math"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/mattn/go-sqlite3"
	"zgo.at/zhttp"
	"zgo.at/zhttp/ctxkey"
)

func init() {
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

func sqlDate(t time.Time) string  { return t.Format("2006-01-02 15:04:05") }
func dayStart(t time.Time) string { return t.Format("2006-01-02") + " 00:00:00" }
func dayEnd(t time.Time) string   { return t.Format("2006-01-02") + " 23:59:59" }

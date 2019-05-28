//go:generate go run pack.go

package goatcounter

import (
	"context"
	"fmt"
	"strings"

	"github.com/jmoiron/sqlx"
	"github.com/mattn/go-sqlite3"
	"zgo.at/zhttp/ctxkey"
)

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

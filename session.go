// Copyright © 2019 Martin Tournoij <martin@arp242.net>
// This file is part of GoatCounter and published under the terms of the EUPL
// v1.2, which can be found in the LICENSE file or at http://eupl12.zgo.at

package goatcounter

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"fmt"
	"sync"
	"time"

	"github.com/pkg/errors"
	"zgo.at/goatcounter/cfg"
	"zgo.at/zdb"
	"zgo.at/zlog"
)

type Session struct {
	ID   int64 `db:"id"`
	Site int64 `db:"site"`

	Hash      []byte    `db:"hash"`
	CreatedAt time.Time `db:"created_at"`
	LastSeen  time.Time `db:"last_seen"`
}

type salt struct {
	mu        sync.Mutex
	today     string
	yesterday string
}

var Salts salt

func (s *salt) Set(today, yesterday string) {
	s.mu.Lock()
	s.today = today
	s.yesterday = yesterday
	s.mu.Unlock()
}

func (s *salt) Get() (today, yesterday string) {
	return s.today, s.yesterday
}

// GetOrCreate gets the session by hash, creating a new one if it doesn't exist
// yet.
func (s *Session) GetOrCreate(ctx context.Context, ua, remoteAddr string) (bool, error) {
	db := zdb.MustGet(ctx)
	site := MustGetSite(ctx)

	td, yd := Salts.Get()

	h := sha256.New()
	h.Write([]byte(fmt.Sprintf("%d%s%s%s", site.ID, ua, remoteAddr, td)))
	hash := h.Sum(nil)

	err := db.GetContext(ctx, s, `select * from sessions where site=$1 and hash=$2`, site.ID, hash)
	if errors.Is(err, sql.ErrNoRows) { // Try yesterday's salt.
		h := sha256.New()
		h.Write([]byte(fmt.Sprintf("%d%s%s%s", site.ID, ua, remoteAddr, yd)))
	}

	err = db.GetContext(ctx, s, `select * from sessions where site=$1 and hash=$2`, site.ID, hash)
	switch err {
	default:
		return false, errors.Wrap(err, "Session.GetOrCreate")

	case nil:
		_, err := db.ExecContext(ctx, `update sessions set last_seen=$1 where site=$2 and hash=$3`,
			Now().Format(zdb.Date), site.ID, hash)
		if err != nil {
			zlog.Error(err)
		}
		return false, nil

	case sql.ErrNoRows:
		s.Site = site.ID
		s.Hash = hash
		s.CreatedAt = Now()
		s.LastSeen = Now()
		query := `insert into sessions (site, hash, created_at, last_seen) values ($1, $2, $3, $4)`
		args := []interface{}{s.Site, s.Hash, s.CreatedAt.Format(zdb.Date), s.LastSeen.Format(zdb.Date)}
		if cfg.PgSQL {
			err := zdb.MustGet(ctx).GetContext(ctx, &s.ID, query+" returning id", args...)
			return true, errors.Wrap(err, "Session.GetOrCreate")
		}

		res, err := zdb.MustGet(ctx).ExecContext(ctx, query, args...)
		if err != nil {
			return true, errors.Wrap(err, "Session.GetOrCreate")
		}
		s.ID, err = res.LastInsertId()
		return true, errors.Wrap(err, "Session.GetOrCreate")
	}
}

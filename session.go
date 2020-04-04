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

	"zgo.at/goatcounter/cfg"
	"zgo.at/goatcounter/errors"
	"zgo.at/zdb"
	"zgo.at/zhttp"
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
	mu       sync.Mutex
	current  string
	previous string
}

var Salts salt

func (s *salt) Set(current, previous string) {
	s.mu.Lock()
	s.current = current
	s.previous = previous
	s.mu.Unlock()
}

func (s *salt) Clear() {
	s.mu.Lock()
	s.current = ""
	s.previous = ""
	s.mu.Unlock()
}

func (s *salt) Get(ctx context.Context) (current, previous string) {
	if s.current == "" || s.previous == "" {
		err := s.Refresh(ctx)
		if err != nil {
			panic(err)
		}
	}

	if s.current == "" {
		panic("salt.Get: s.current is empty")
	}
	if s.previous == "" {
		panic("salt.Get: s.previous is empty")
	}
	return s.current, s.previous
}

func (s *salt) Refresh(ctx context.Context) error {
	var newsalt []struct {
		Salt      string    `db:"salt"`
		CreatedAt time.Time `db:"created_at"`
	}

	err := zdb.TX(ctx, func(ctx context.Context, db zdb.DB) error {
		err := db.SelectContext(ctx, &newsalt,
			`select salt, created_at from session_salts order by previous asc`)
		if err != nil {
			return err
		}

		if len(newsalt) == 0 { // First run
			_, err = db.ExecContext(ctx, `insert into session_salts (previous, salt, created_at)
				values (0, $1, $2), (1, $3, $4)`,
				zhttp.Secret(), Now().Format(zdb.Date), zhttp.Secret(), Now().Format(zdb.Date))
			if err != nil {
				return err
			}
			goto get
		}

		if newsalt[1].CreatedAt.Add(12 * time.Hour).After(Now()) {
			goto get
			return nil
		}

		_, err = db.ExecContext(ctx, `delete from session_salts where previous=1`)
		if err != nil {
			return err
		}

		_, err = db.ExecContext(ctx, `update session_salts set previous=1 where previous=0`)
		if err != nil {
			return err
		}

		_, err = db.ExecContext(ctx, `insert into session_salts (previous, salt, created_at) values (0, $1, $2)`,
			zhttp.Secret(), Now())
		if err != nil {
			return err
		}

	get:
		newsalt = newsalt[0:0]
		err = db.SelectContext(ctx, &newsalt, `select salt from session_salts order by previous asc`)
		if err != nil {
			return err
		}
		s.Set(newsalt[0].Salt, newsalt[1].Salt)
		return nil
	})
	if err != nil {
		return fmt.Errorf("salt.Refresh: %w", err)
	}

	return nil
}

// GetOrCreate gets the session by hash, creating a new one if it doesn't exist
// yet.
func (s *Session) GetOrCreate(ctx context.Context, ua, remoteAddr string) (bool, error) {
	db := zdb.MustGet(ctx)
	site := MustGetSite(ctx)
	now := Now()
	curSalt, prevSalt := Salts.Get(ctx)

	h := sha256.New()
	h.Write([]byte(fmt.Sprintf("%d%s%s%s", site.ID, ua, remoteAddr, curSalt)))
	hash := h.Sum(nil)

	err := db.GetContext(ctx, s, `select * from sessions where site=$1 and hash=$2`, site.ID, hash)
	if zdb.ErrNoRows(err) { // Try previous salt.
		h := sha256.New()
		h.Write([]byte(fmt.Sprintf("%d%s%s%s", site.ID, ua, remoteAddr, prevSalt)))
		prevHash := h.Sum(nil)

		err = db.GetContext(ctx, s, `select * from sessions where site=$1 and hash=$2`, site.ID, prevHash)
		if err == nil {
			hash = prevHash
		}
	}

	switch err {
	default:
		return false, errors.Wrap(err, "Session.GetOrCreate")

	case nil:
		_, err := db.ExecContext(ctx, `update sessions set last_seen=$1 where site=$2 and hash=$3`,
			now.Format(zdb.Date), site.ID, hash)
		if err != nil {
			zlog.Error(err)
		}
		return false, nil

	case sql.ErrNoRows:
		s.Site = site.ID
		s.Hash = hash
		s.CreatedAt = now
		s.LastSeen = now
		query := `insert into sessions (site, hash, created_at, last_seen) values ($1, $2, $3, $4)`
		args := []interface{}{s.Site, s.Hash, s.CreatedAt.Format(zdb.Date), s.LastSeen.Format(zdb.Date)}
		if cfg.PgSQL {
			err := zdb.MustGet(ctx).GetContext(ctx, &s.ID, query+" returning id", args...)
			return true, errors.Wrap(err, "Session.GetOrCreate")
		}

		// SQLite
		res, err := zdb.MustGet(ctx).ExecContext(ctx, query, args...)
		if err != nil {
			return true, errors.Wrap(err, "Session.GetOrCreate")
		}
		s.ID, err = res.LastInsertId()
		return true, errors.Wrap(err, "Session.GetOrCreate")
	}
}

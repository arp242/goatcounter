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

	"zgo.at/errors"
	"zgo.at/goatcounter/cfg"
	"zgo.at/zdb"
	"zgo.at/zhttp"
	"zgo.at/zlog"
	"zgo.at/zstd/zsync"
)

type Session struct {
	ID   int64 `db:"id"`
	Site int64 `db:"site"`

	Hash      []byte    `db:"hash"`
	CreatedAt time.Time `db:"created_at"`
	LastSeen  time.Time `db:"last_seen"`
}

type Salt struct {
	mu         sync.Mutex
	current    string
	previous   string
	CycleEvery time.Duration

	firstRefresh sync.Once
}

var Salts = Salt{
	CycleEvery: 4 * time.Hour,
}

func (s *Salt) Set(current, previous string) {
	s.mu.Lock()
	s.current = current
	s.previous = previous
	s.mu.Unlock()
}

func (s *Salt) Clear() {
	s.mu.Lock()
	s.current = ""
	s.previous = ""
	s.firstRefresh = sync.Once{}
	s.mu.Unlock()
}

func (s *Salt) Get(ctx context.Context) (current, previous string) {
	if s.current == "" || s.previous == "" {
		s.firstRefresh.Do(func() {
			err := s.Refresh(ctx)
			if err != nil {
				panic(err)
			}
		})
	}

	if s.current == "" {
		panic("Salt.Get: s.current is empty")
	}
	if s.previous == "" {
		panic("Salt.Get: s.previous is empty")
	}
	return s.current, s.previous
}

func (s *Salt) Refresh(ctx context.Context) error {
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

		if s.current == "" { // Server restart
			goto get
		}

		if newsalt[1].CreatedAt.Add(s.CycleEvery).After(Now()) {
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
		return errors.Errorf("salt.Refresh: %w", err)
	}

	return nil
}

// hasPath reports if this session has already visited a path.
func (s *Session) hasPath(ctx context.Context, path string) (bool, error) {
	if s.ID == 0 {
		return false, fmt.Errorf("Session.hasPath: s.ID is 0")
	}

	var r uint8
	err := zdb.MustGet(ctx).GetContext(ctx, &r, `/* Session.hasPath */
		select 1 from session_paths where session=$1 and lower(path) = lower($2) limit 1`,
		s.ID, path)
	if zdb.ErrNoRows(err) {
		return false, nil
	}
	return true, errors.Wrap(err, "Session.hasPath")
}

// GetOrCreate gets the session by hash, creating a new one if it doesn't exist
// yet.
func (s *Session) GetOrCreate(ctx context.Context, path, ua, remoteAddr string) (firstVisit bool, err error) {
	return s.getOrCreate(ctx, path, ua, remoteAddr, 0)
}

var hashOnce zsync.Once

func (s *Session) getOrCreate(ctx context.Context, path, ua, remoteAddr string, r int) (firstVisit bool, err error) {
	if r > 10 {
		zlog.Module("session").Fields(zlog.F{
			"remoteAddr": remoteAddr,
			"ua":         ua,
			"siteID":     MustGetSite(ctx).ID,
		}).Printf("recurse > 10")
		return false, nil
	}

	db := zdb.MustGet(ctx)
	site := MustGetSite(ctx)
	now := Now()
	curSalt, prevSalt := Salts.Get(ctx)

	h := sha256.New()
	h.Write([]byte(fmt.Sprintf("%d%s%s%s", site.ID, ua, remoteAddr, curSalt)))
	hash := h.Sum(nil)

	err = db.GetContext(ctx, s, `select * from sessions where site=$1 and hash=$2`, site.ID, hash)
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
		_, err := db.ExecContext(ctx, `update sessions
			set last_seen=$1 where site=$2 and hash=$3`,
			now.Format(zdb.Date), site.ID, hash)
		if err != nil {
			zlog.Errorf("Session.getOrCreate: update: %w", err)
		}

		has, err := s.hasPath(ctx, path)
		if err != nil {
			zlog.Errorf("Session.getOrCreate: %w", err)
			return false, nil
		}

		if !has {
			_, err = db.ExecContext(ctx, `
				insert into session_paths (session, path) values ($1, $2)`,
				s.ID, path)
			if err != nil {
				zlog.Errorf("Session.getOrCreate: insert path: %w", err)
			}
		}

		return !has, nil

	case sql.ErrNoRows:
		var err error
		hh := fmt.Sprintf("%x", hash)
		ran := hashOnce.Do(hh, func() {
			s.Site = site.ID
			s.Hash = hash
			s.CreatedAt = now
			s.LastSeen = now
			err = s.create(ctx, path)
		})
		if !ran {
			time.Sleep(50 * time.Millisecond)
			return s.getOrCreate(ctx, path, ua, remoteAddr, r+1)
		}
		if err != nil {
			return false, err
		}
		go func() {
			time.Sleep(10 * time.Second)
			hashOnce.Forget(hh)
		}()
		return true, nil
	}
}

func (s *Session) create(ctx context.Context, path string) error {
	query := `insert into sessions (site, hash, created_at, last_seen) values ($1, $2, $3, $4)`
	args := []interface{}{s.Site, s.Hash, s.CreatedAt.Format(zdb.Date), s.LastSeen.Format(zdb.Date)}

	db := zdb.MustGet(ctx)

	if cfg.PgSQL {
		err := db.GetContext(ctx, &s.ID, query+" returning id", args...)
		if err != nil {
			return errors.Wrap(err, "Session.create: insert")
		}
	} else {
		res, err := db.ExecContext(ctx, query, args...)
		if err != nil {
			return errors.Errorf("Session.create: insert: %w", err)
		}
		s.ID, err = res.LastInsertId()
		if err != nil {
			return errors.Errorf("Session.create: lastInsertID: %w", err)
		}
	}

	_, err := db.ExecContext(ctx, `insert into session_paths (session, path) values ($1, $2)`, s.ID, path)
	return errors.Wrap(err, "Session.create: insert path")
}

// Copyright © 2019 Martin Tournoij – This file is part of GoatCounter and
// published under the terms of a slightly modified EUPL v1.2 license, which can
// be found in the LICENSE file or at https://license.goatcounter.com

package gomig

import (
	"context"
	"time"

	"zgo.at/zdb"
)

type storedSession struct {
	Sessions    map[string]int64              `json:"sessions"`
	Hashes      map[int64]string              `json:"hashes"`
	Paths       map[int64]map[string]struct{} `json:"paths"`
	Seen        map[int64]int64               `json:"seen"`
	CurSalt     []byte                        `json:"cur_salt"`
	PrevSalt    []byte                        `json:"prev_salt"`
	SaltRotated time.Time                     `json:"salt_rotated"`
}

func MemSess(db zdb.DB) error {
	return zdb.TX(zdb.With(context.Background(), db), func(ctx context.Context, db zdb.DB) (retErr error) {
		var err error
		defer func() {
			if err == nil {
				_, retErr = db.ExecContext(ctx, `insert into version values ('2020-07-22-1-memsess')`)
			}
		}()

		// Populate store with existing data.
		var stored storedSession
		var sessions []struct {
			ID       int64     `db:"id"`
			Hash     string    `db:"hash"`
			LastSeen time.Time `db:"last_seen"`
		}
		err = db.SelectContext(ctx, &sessions, `select id, hash, last_seen from sessions`)
		if err != nil {
			return err
		}

		var paths []struct {
			Session int64  `db:"session"`
			Path    string `db:"path"`
		}
		err = db.SelectContext(ctx, &paths, `select session, path from session_paths`)
		if err != nil {
			return err
		}

		var salts []struct {
			Previous  int       `db:"previous"`
			Salt      string    `db:"salt"`
			CreatedAt time.Time `db:"created_at"`
		}
		err = db.SelectContext(ctx, &salts, `select previous, salt, created_at from session_salts`)
		if err != nil {
			return err
		}

		for _, s := range salts {
			if s.Previous == 1 {
				stored.PrevSalt = []byte(s.Salt)
			} else {
				stored.CurSalt = []byte(s.Salt)
				stored.SaltRotated = s.CreatedAt.UTC()
			}
		}

		stored.Paths = make(map[int64]map[string]struct{}, len(paths))
		for _, p := range paths {
			if stored.Paths[p.Session] == nil {
				stored.Paths[p.Session] = make(map[string]struct{})
			}
			stored.Paths[p.Session][p.Path] = struct{}{}
		}

		stored.Sessions = make(map[string]int64, len(sessions))
		stored.Hashes = make(map[int64]string, len(sessions))
		stored.Seen = make(map[int64]int64, len(sessions))
		for _, s := range sessions {
			stored.Sessions[s.Hash] = s.ID
			stored.Hashes[s.ID] = s.Hash
			stored.Seen[s.ID] = s.LastSeen.Unix()
		}

		// Drop old tables.
		_, err = db.ExecContext(ctx, `drop table session_salts`)
		if err != nil {
			return err
		}
		_, err = db.ExecContext(ctx, `drop table session_paths`)
		if err != nil {
			return err
		}
		_, err = db.ExecContext(ctx, `drop table sessions`)
		if err != nil {
			return err
		}

		// Add new column for new session type.
		if zdb.PgSQL(db) {
			_, err = db.ExecContext(ctx, `alter table hits add column session2 bytea not null default ''`)
		} else {
			_, err = db.ExecContext(ctx, `alter table hits add column session2 blob not null default ''`)
		}
		if err != nil {
			return err
		}

		return
	})
}

// Copyright © 2019 Martin Tournoij – This file is part of GoatCounter and
// published under the terms of a slightly modified EUPL v1.2 license, which can
// be found in the LICENSE file or at https://license.goatcounter.com

package gomig

import (
	"context"

	"zgo.at/errors"
	"zgo.at/zdb"
	"zgo.at/zlog"
	"zgo.at/zstd/zstring"
)

var goMigrations = map[string]func(zdb.DB) error{
	"2020-03-27-1-isbot":       IsBot,
	"2020-07-22-1-memsess":     MemSess,
	"2020-08-28-4-user_agents": UserAgents,
}

func Run(db zdb.DB) error {
	var ran []string
	err := db.SelectContext(context.Background(), &ran,
		`select name from version order by name asc`)
	if err != nil {
		return errors.Errorf("runGoMigrations: %w", err)
	}

	ctx := zdb.With(context.Background(), db)

	for k, f := range goMigrations {
		if zstring.Contains(ran, k) {
			continue
		}
		zlog.Printf("running Go migration %q", k)

		err := zdb.TX(ctx, func(ctx context.Context, db zdb.DB) error {
			err := f(db)
			if err != nil {
				return errors.Errorf("runGoMigrations: running migration %q: %w", k, err)
			}

			_, err = db.ExecContext(context.Background(), `insert into version values ($1)`, k)
			if err != nil {
				return errors.Errorf("runGoMigrations: update version: %w", err)
			}
			return nil
		})
		if err != nil {
			return err
		}
	}
	return nil
}

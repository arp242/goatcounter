// Copyright © 2019 Martin Tournoij <martin@arp242.net>
// This file is part of GoatCounter and published under the terms of the EUPL
// v1.2, which can be found in the LICENSE file or at http://eupl12.zgo.at

package cron

import (
	"context"
	"fmt"
	"time"

	"zgo.at/goatcounter"
	"zgo.at/zdb"
	"zgo.at/zhttp"
)

func updateSalts(ctx context.Context) error {
	var newsalt []struct {
		Salt      string    `db:"salt"`
		CreatedAt time.Time `db:"created_at"`
	}

	err := zdb.TX(ctx, func(ctx context.Context, db zdb.DB) error {
		err := db.SelectContext(ctx, &newsalt, `select salt from session_salts order by key asc`)
		if err != nil {
			return err
		}

		if len(newsalt) == 0 { // First run
			_, err = db.ExecContext(ctx, `insert into session_salts (key, salt, created_at)
				values (0, $1, $2), (0, $3, $4)`, zhttp.Secret(), goatcounter.Now(),
				zhttp.Secret(), goatcounter.Now().Add(-24*time.Hour))
			return err
		}

		if newsalt[0].CreatedAt.Before(goatcounter.Now().Add(24 * time.Hour)) {
			return nil
		}

		_, err = db.ExecContext(ctx, `delete from session_salts where key>0`)
		if err != nil {
			return err
		}

		_, err = db.ExecContext(ctx, `update session_salts set key=1 where key=0`)
		if err != nil {
			return err
		}

		_, err = db.ExecContext(ctx, `insert into session_salts (key, salt, created_at) values (0, $1, $2)`,
			zhttp.Secret(), goatcounter.Now())
		if err != nil {
			return err
		}

		return db.SelectContext(ctx, &newsalt, `select salt from session_salts order by key asc`)
	})
	if err != nil {
		return fmt.Errorf("updateSalts: %w", err)
	}

	goatcounter.Salts.Set(newsalt[0].Salt, newsalt[1].Salt)
	return nil
}

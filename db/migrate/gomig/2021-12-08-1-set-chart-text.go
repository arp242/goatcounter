package gomig

import (
	"context"

	"zgo.at/errors"
	"zgo.at/goatcounter/v2"
	"zgo.at/json"
	"zgo.at/zdb"
)

func KeepAsText(ctx context.Context) error {
	// Not implemented for SQLite.
	if zdb.Driver(ctx) == zdb.DriverSQLite {
		return nil
	}

	set := func(settings []byte) (string, error) {
		var s map[string]interface{}
		err := json.Unmarshal(settings, &s)
		if err != nil {
			return "", err
		}

		wid := s["widgets"].([]interface{})
		for i, w := range wid {
			ww := w.(map[string]interface{})
			if ww["n"].(string) == "pages" {
				s, ok := ww["s"].(map[string]interface{})
				if !ok {
					s = make(map[string]interface{})
				}
				s["style"] = "text"
				ww["s"] = s
				wid[i] = ww
				break
			}
		}
		s["widgets"] = wid

		j, err := json.MarshalIndent(s, "", "    ")
		return string(j), err
	}

	err := zdb.TX(goatcounter.NewCache(goatcounter.NewConfig(ctx)), func(ctx context.Context) error {
		// Update user settings.
		var users []struct {
			ID       int64  `db:"user_id"`
			Settings []byte `db:"settings"`
		}
		err := zdb.Select(ctx, &users, `select user_id, settings from users
			where settings->'views'->0->'as-text' = 'true'`)
		if err != nil {
			return err
		}

		for _, u := range users {
			s, err := set(u.Settings)
			if err != nil {
				return errors.Wrapf(err, "user %d", u.ID)
			}
			err = zdb.Exec(ctx, `update users set settings=? where user_id=?`, s, u.ID)
			if err != nil {
				return errors.Wrapf(err, "user %d", u.ID)
			}
		}

		// Update site settings.
		var sites []struct {
			ID           int64  `db:"site_id"`
			UserDefaults []byte `db:"user_defaults"`
		}
		err = zdb.Select(ctx, &sites, `select site_id, user_defaults from sites
			where user_defaults->'views'->0->'as-text' = 'true'`)
		if err != nil {
			return err
		}

		for _, site := range sites {
			s, err := set(site.UserDefaults)
			if err != nil {
				return errors.Wrapf(err, "site %d", site.ID)
			}
			err = zdb.Exec(ctx, `update sites set user_defaults=? where site_id=?`, s, site.ID)
			if err != nil {
				return errors.Wrapf(err, "site %d", site.ID)
			}
		}

		return nil
	})

	if err == nil {
		err = zdb.Exec(ctx, `insert into version values ('2021-12-08-1-set-chart-text')`)
	}
	return err
}

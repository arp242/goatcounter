// Copyright © 2019 Martin Tournoij – This file is part of GoatCounter and
// published under the terms of a slightly modified EUPL v1.2 license, which can
// be found in the LICENSE file or at https://license.goatcounter.com

package gomig

import (
	"context"

	"zgo.at/errors"
	"zgo.at/goatcounter"
	"zgo.at/json"
	"zgo.at/zdb"
)

// Can probably do this with JSON in SQL too, but ugh.
func Widgets(ctx context.Context) error {
	err := zdb.TX(goatcounter.NewCache(goatcounter.NewConfig(ctx)), func(ctx context.Context) error {
		var users []struct {
			ID       int64  `db:"user_id"`
			Site     int64  `db:"site_id"`
			Settings []byte `db:"settings"`
		}
		err := zdb.Select(ctx, &users, `select user_id, site_id, settings from users`)
		if err != nil {
			return err
		}

		for _, u := range users {
			var site goatcounter.Site
			err = site.ByID(ctx, u.Site)
			if err != nil {
				err = site.ByIDState(ctx, u.Site, goatcounter.StateDeleted)
				if err != nil {
					return err
				}
			}
			ctx = goatcounter.WithSite(ctx, &site)
			var user goatcounter.User
			err = user.ByID(ctx, u.ID)
			if err != nil {
				return err
			}

			{
				var s map[string]interface{}
				err := json.Unmarshal(u.Settings, &s)
				if err != nil {
					return errors.Wrapf(err, "%d", u.ID)
				}

				var wid goatcounter.Widgets
				for _, v := range s["widgets"].([]interface{}) {
					vv := v.(map[string]interface{})
					w := goatcounter.Widget{"n": vv["name"]}
					if vv["s"] != nil && len(vv["s"].(map[string]interface{})) > 0 {
						w["s"] = vv["s"]
					}
					wid = append(wid, w)
				}

				s["widgets"] = wid
				ss, err := json.Marshal(s)
				if err != nil {
					return err
				}

				err = zdb.Exec(ctx, `update users set settings=? where user_id=?`, ss, u.ID)
				if err != nil {
					return errors.Wrapf(err, "set user %d", u.ID)
				}
			}
			{
				var wid goatcounter.Widgets
				for _, v := range site.UserDefaults.Widgets {
					w := goatcounter.Widget{"n": v["name"]}
					if v["s"] != nil && len(v["s"].(map[string]interface{})) > 0 {
						w["s"] = v["s"]
					}
					wid = append(wid, w)
				}
				site.UserDefaults.Widgets = wid

				err = zdb.Exec(ctx, `update sites set user_defaults=? where site_id=?`, site.UserDefaults, site.ID)
				if err != nil {
					return err
				}
			}
		}
		return nil
	})

	if err == nil {
		err = zdb.Exec(ctx, `insert into version values ('2021-03-29-1-widgets')`)
	}
	return err
}

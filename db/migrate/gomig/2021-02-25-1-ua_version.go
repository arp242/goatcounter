// Copyright © 2019 Martin Tournoij – This file is part of GoatCounter and
// published under the terms of a slightly modified EUPL v1.2 license, which can
// be found in the LICENSE file or at https://license.goatcounter.com

package gomig

import (
	"context"
	"strings"

	"zgo.at/goatcounter/v2"
	"zgo.at/zdb"
)

func UserAgentVersion(ctx context.Context) error {
	// Systems
	var systems []goatcounter.System
	err := zdb.Select(ctx, &systems,
		`select * from systems where version != '' and name not in ('Linux', 'Windows') order by system_id`)
	if err != nil {
		return err
	}

	for _, s := range systems {
		n := toNumber(s.Version)
		if n == s.Version {
			continue
		}

		oldID := s.ID
		err = s.GetOrInsert(ctx, s.Name, n)
		if err != nil {
			return err
		}

		err = zdb.Exec(ctx, `update user_agents set system_id = $1 where system_id = $2`, s.ID, oldID)
		if err != nil {
			if !zdb.ErrUnique(err) {
				return err
			}
			err = nil
		}

		// Just delete these; they're not many and it's a bit tricky to
		// update correctly.
		err = zdb.Exec(ctx, `delete from system_stats where system_id = $1`, oldID)
		if err != nil {
			return err
		}
		err = zdb.Exec(ctx, `delete from systems where system_id = $1`, oldID)
		if err != nil {
			return err
		}
	}

	// Browsers
	var browsers []goatcounter.Browser
	err = zdb.Select(ctx, &browsers,
		`select * from browsers where version != '' order by browser_id`)
	if err != nil {
		return err
	}

	for _, s := range browsers {
		n := toNumber(s.Version)
		if n == s.Version {
			continue
		}

		oldID := s.ID
		err = s.GetOrInsert(ctx, s.Name, n)
		if err != nil {
			return err
		}

		err = zdb.Exec(ctx, `update user_agents set browser_id = $1 where browser_id = $2`, s.ID, oldID)
		if err != nil {
			if !zdb.ErrUnique(err) {
				return err
			}
			err = nil
		}

		// Just delete these; they're not many and it's a bit tricky to
		// update correctly.
		err = zdb.Exec(ctx, `delete from browser_stats where browser_id = $1`, oldID)
		if err != nil {
			return err
		}
		err = zdb.Exec(ctx, `delete from browsers where browser_id = $1`, oldID)
		if err != nil {
			return err
		}
	}

	return nil
}

func toNumber(v string) string {
	var b strings.Builder
	for _, r := range v {
		if !(r == '.' || (r >= '0' && r <= '9')) {
			break
		}
		b.WriteRune(r)
	}

	return b.String()
}

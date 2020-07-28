// Copyright © 2019 Martin Tournoij – This file is part of GoatCounter and
// published under the terms of a slightly modified EUPL v1.2 license, which can
// be found in the LICENSE file or at https://license.goatcounter.com

package gomig

import (
	"context"
	"fmt"

	"zgo.at/errors"
	"zgo.at/gadget"
	"zgo.at/goatcounter"
	"zgo.at/zdb"
	"zgo.at/zli"
)

func UserAgents(db zdb.DB) error {
	ctx := zdb.With(context.Background(), db)

	var agents []struct {
		ID        int64  `db:"user_agent_id"`
		UserAgent string `db:"ua"`
	}
	err := db.SelectContext(ctx, &agents,
		`select user_agent_id, ua from user_agents order by user_agent_id asc`)
	if err != nil {
		return err
	}

	if len(agents) == 0 {
		return nil
	}

	errs := errors.NewGroup(1000)
	for i, u := range agents {
		if i%100 == 0 {
			zli.ReplaceLinef("Progress: %d/%d", i, len(agents))
		}
		ua := gadget.Parse(u.UserAgent)

		var browser goatcounter.Browser
		err := browser.GetOrInsert(ctx, ua.BrowserName, ua.BrowserVersion)
		if err != nil {
			errs.Append(err)
			continue
		}

		var system goatcounter.System
		err = system.GetOrInsert(ctx, ua.OSName, ua.OSVersion)
		if err != nil {
			errs.Append(err)
			continue
		}

		_, err = db.ExecContext(ctx, `update user_agents
			set browser_id=$1, system_id=$2, ua=$3 where user_agent_id=$4`,
			browser.ID, system.ID, gadget.Shorten(u.UserAgent), u.ID)
		errs.Append(err)
	}
	if errs.Len() > 0 {
		return errs
	}

	fmt.Println("\nDone!")
	return nil
}

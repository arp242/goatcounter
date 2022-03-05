// Copyright © Martin Tournoij – This file is part of GoatCounter and published
// under the terms of a slightly modified EUPL v1.2 license, which can be found
// in the LICENSE file or at https://license.goatcounter.com

package gomig

import (
	"context"
	"fmt"
	"strings"

	"zgo.at/errors"
	"zgo.at/gadget"
	"zgo.at/goatcounter/v2"
	"zgo.at/isbot"
	"zgo.at/zdb"
	"zgo.at/zli"
)

type agentsT []struct {
	ID        int64  `db:"user_agent_id"`
	UserAgent string `db:"ua"`
}

func getAgents(ctx context.Context) (agentsT, error) {
	var agents agentsT
	err := zdb.Select(ctx, &agents, `select user_agent_id, ua from user_agents order by user_agent_id asc`)
	return agents, err
}

func UserAgents(ctx context.Context) error {
	agents, err := getAgents(ctx)
	if err != nil {
		return err
	}
	if len(agents) == 0 {
		return nil
	}

	// Remove duplicates first.
	deleted := make(map[int64]struct{})
	for _, u := range agents {
		if _, ok := deleted[u.ID]; ok {
			continue
		}

		if strings.ContainsRune(u.UserAgent, '~') {
			u.UserAgent = gadget.Unshorten(u.UserAgent)
		} else {
			u.UserAgent = gadget.Shorten(u.UserAgent)
		}

		var dupes []int64
		err := zdb.Select(ctx, &dupes, `select user_agent_id from user_agents where ua=? and user_agent_id != ?`,
			u.UserAgent, u.ID)
		if err != nil {
			return err
		}
		if len(dupes) > 0 {
			fmt.Printf("%d → dupes: %v\n", u.ID, dupes)
			err := zdb.Exec(ctx, `update hits set user_agent_id=? where user_agent_id in (?)`, u.ID, dupes)
			if err != nil {
				return err
			}
			err = zdb.Exec(ctx, `delete from user_agents where user_agent_id in (?)`, dupes)
			if err != nil {
				return err
			}
			for _, d := range dupes {
				deleted[d] = struct{}{}
			}
		}
	}

	agents, err = getAgents(ctx)
	if err != nil {
		return err
	}

	errs := errors.NewGroup(1000)
	for i, u := range agents {
		if i%100 == 0 {
			zli.ReplaceLinef("Progress: %d/%d", i, len(agents))
		}

		if strings.ContainsRune(u.UserAgent, '~') {
			u.UserAgent = gadget.Unshorten(u.UserAgent)
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

		bot := isbot.UserAgent(u.UserAgent)
		err = zdb.Exec(ctx, `update user_agents
				set browser_id=$1, system_id=$2, ua=$3, isbot=$4 where user_agent_id=$5`,
			browser.ID, system.ID, gadget.Shorten(u.UserAgent), bot, u.ID)
		errs.Append(errors.Wrapf(err, "update user_agent %d", u.ID))
	}
	if errs.Len() > 0 {
		return errs
	}

	fmt.Println("\nDone!")
	return nil
}

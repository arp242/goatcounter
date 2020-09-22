// Copyright © 2019 Martin Tournoij – This file is part of GoatCounter and
// published under the terms of a slightly modified EUPL v1.2 license, which can
// be found in the LICENSE file or at https://license.goatcounter.com

package gomig

import (
	"zgo.at/zdb"
)

var Migrations = map[string]func(zdb.DB) error{
	"2020-07-22-1-memsess":     MemSess,
	"2020-08-28-4-user_agents": UserAgents,
}

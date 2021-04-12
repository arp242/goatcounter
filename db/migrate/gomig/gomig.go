// Copyright © 2019 Martin Tournoij – This file is part of GoatCounter and
// published under the terms of a slightly modified EUPL v1.2 license, which can
// be found in the LICENSE file or at https://license.goatcounter.com

package gomig

import "context"

var Migrations = map[string]func(context.Context) error{
	"2020-12-31-1-user_agents": UserAgents,
	"2021-02-25-1-ua_version":  UserAgentVersion,
	"2021-03-29-1-widgets":     Widgets,
}

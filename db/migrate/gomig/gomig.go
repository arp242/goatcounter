// Copyright © Martin Tournoij – This file is part of GoatCounter and published
// under the terms of a slightly modified EUPL v1.2 license, which can be found
// in the LICENSE file or at https://license.goatcounter.com

package gomig

import "context"

var Migrations = map[string]func(context.Context) error{
	"2021-12-08-1-set-chart-text":    KeepAsText,
	"2022-11-15-1-correct-hit-stats": CorrectHitStats,
}

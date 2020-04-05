// Copyright © 2019 Martin Tournoij <martin@arp242.net>
// This file is part of GoatCounter and published under the terms of the EUPL
// v1.2, which can be found in the LICENSE file or at http://eupl12.zgo.at

package goatcounter

import (
	"context"

	"zgo.at/goatcounter/errors"
	"zgo.at/zdb"
	"zgo.at/zlog"
)

// HasFlag checks if this flag is enabled for the current site.
func HasFlag(ctx context.Context, name string) bool {
	siteID := MustGetSite(ctx).ID
	if siteID == 0 {
		zlog.Errorf("HasFlag: site.ID is 0")
		return false
	}

	var ok bool
	err := zdb.MustGet(ctx).GetContext(ctx, &ok,
		`select 1 from flags where name=$1 and value in (0, $2)`,
		name, siteID)
	if zdb.ErrNoRows(err) {
		return false
	}
	if err != nil {
		zlog.Fields(zlog.F{
			"name":   name,
			"siteID": siteID,
		}).Error(errors.Wrap(err, "HasFlag"))
		return false
	}
	return ok
}

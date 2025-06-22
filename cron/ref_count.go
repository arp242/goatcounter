package cron

import (
	"context"
	"strconv"

	"zgo.at/errors"
	"zgo.at/goatcounter/v2"
	"zgo.at/zdb"
)

func updateRefCounts(ctx context.Context, hits []goatcounter.Hit) error {
	err := zdb.TX(ctx, func(ctx context.Context) error {
		// Group by day + pathID + ref.
		type gt struct {
			total  int
			hour   string
			pathID goatcounter.PathID
			refID  goatcounter.RefID
		}
		grouped := map[string]gt{}
		for _, h := range hits {
			if h.Bot > 0 {
				continue
			}

			hour := h.CreatedAt.Format("2006-01-02 15:00:00")
			k := hour + strconv.Itoa(int(h.PathID)) + strconv.Itoa(int(h.RefID))
			v := grouped[k]
			if v.total == 0 {
				v.hour = hour
				v.pathID = h.PathID
				v.refID = h.RefID
			}

			if h.FirstVisit {
				v.total += 1
			}
			grouped[k] = v
		}

		var (
			siteID = goatcounter.MustGetSite(ctx).ID
			ins    = goatcounter.Tables.RefCounts.Bulk(ctx)
		)
		for _, v := range grouped {
			if v.total > 0 {
				ins.Values(siteID, v.pathID, v.hour, v.refID, v.total)
			}
		}
		return ins.Finish()
	})
	return errors.Wrap(err, "cron.updateRefCounts")
}

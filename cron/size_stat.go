package cron

import (
	"context"
	"strconv"

	"zgo.at/errors"
	"zgo.at/goatcounter/v2"
	"zgo.at/zdb"
)

func updateSizeStats(ctx context.Context, hits []goatcounter.Hit) error {
	err := zdb.TX(ctx, func(ctx context.Context) error {
		type gt struct {
			count  int
			day    string
			width  int
			pathID goatcounter.PathID
		}
		grouped := map[string]gt{}
		for _, h := range hits {
			if h.Bot > 0 {
				continue
			}

			var width int
			if len(h.Size) > 0 {
				width = int(h.Size[0])
			}

			day := h.CreatedAt.Format("2006-01-02")
			k := day + strconv.Itoa(width) + strconv.Itoa(int(h.PathID))
			v := grouped[k]
			if v.count == 0 {
				v.day = day
				v.width = width
				v.pathID = h.PathID
			}

			if h.FirstVisit {
				v.count += 1
			}
			grouped[k] = v
		}

		ins, err := goatcounter.Tables.SizeStats.Bulk(ctx)
		if err != nil {
			return err
		}

		siteID := goatcounter.MustGetSite(ctx).ID
		for _, v := range grouped {
			if v.count > 0 {
				ins.Values(siteID, v.pathID, v.day, v.width, v.count)
			}
		}
		return ins.Finish()
	})
	return errors.Wrap(err, "cron.updateSizeStats")
}

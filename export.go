// Copyright © 2019 Martin Tournoij <martin@arp242.net>
// This file is part of GoatCounter and published under the terms of the EUPL
// v1.2, which can be found in the LICENSE file or at http://eupl12.zgo.at

package goatcounter

import (
	"compress/gzip"
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"net/mail"
	"os"
	"time"

	"zgo.at/utils/sliceutil"
	"zgo.at/zdb"
	"zgo.at/zhttp/zmail"
	"zgo.at/zlog"
)

func ExportFile(site *Site) string {
	return fmt.Sprintf("%s/goatcounter-export-%s.csv.gz", os.TempDir(), site.Code)
}

// Export all data to a CSV file.
//
// TODO: cron job to remove these files.
func Export(ctx context.Context, fp io.WriteCloser) {
	defer fp.Close()

	gzfp := gzip.NewWriter(fp)
	defer gzfp.Close()

	c := csv.NewWriter(gzfp)
	c.Write([]string{"Path", "Title", "Event", "Bot", "Session",
		"Referrer (sanitized)", "Referrer query params",
		"Original Referrer", "Browser", "Screen size", "Location",
		"Date"})

	l := zlog.Module("export").Field("site", MustGetSite(ctx).ID)
	var (
		last int64
		err  error
	)
	for {
		var hits Hits
		last, err = hits.List(ctx, 5000, last)
		if zdb.ErrNoRows(err) {
			// TODO: better.
			zmail.Send("GoatCounter export ready",
				mail.Address{Name: "GoatCounter export", Address: "support@goatcounter.com"},
				[]mail.Address{{Address: "support@goatcounter.com"}},
				fmt.Sprintf(""))
			break
		}
		if err != nil {
			l.Error(err)
			break
		}

		for _, hit := range hits {
			rp := ""
			if hit.RefParams != nil {
				rp = *hit.RefParams
			}
			ro := ""
			if hit.RefOriginal != nil {
				ro = *hit.RefOriginal
			}
			c.Write([]string{hit.Path, hit.Title, fmt.Sprintf("%t", hit.Event),
				fmt.Sprintf("%d", hit.Bot), fmt.Sprintf("%d", hit.Session),
				hit.Ref, rp, ro, hit.Browser, sliceutil.JoinFloat(hit.Size),
				hit.Location, hit.CreatedAt.Format(time.RFC3339)})
		}

		c.Flush()
		err = c.Error()
		if err != nil {
			l.Error(err)
			break
		}
	}
}

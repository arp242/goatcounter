// Copyright © 2019 Martin Tournoij <martin@arp242.net>
// This file is part of GoatCounter and published under the terms of the EUPL
// v1.2, which can be found in the LICENSE file or at http://eupl12.zgo.at

package goatcounter

import (
	"compress/gzip"
	"context"
	"encoding/csv"
	"fmt"
	"net/mail"
	"os"
	"time"

	"zgo.at/goatcounter/cfg"
	"zgo.at/utils/sliceutil"
	"zgo.at/zhttp/zmail"
	"zgo.at/zlog"
)

const emailExportDone = `Hi there,

The GoatCounter export you’ve requested is finished, go here to download it:
%s/download-export

The file size is %.1fM, and the export will be removed after 24 hours.

Feel free to reply to this email if you have any questions or problems.
`

func ExportFile(site *Site) string {
	return fmt.Sprintf("%s/goatcounter-export-%s.csv.gz", os.TempDir(), site.Code)
}

// Export all data to a CSV file.
func Export(ctx context.Context, fp *os.File) {
	site := MustGetSite(ctx)
	l := zlog.Module("export").Field("site", site.ID)
	l.Print("export started")

	gzfp := gzip.NewWriter(fp)
	defer fp.Close() // No need to error-check; just for safety.
	defer gzfp.Close()

	c := csv.NewWriter(gzfp)
	c.Write([]string{"Path", "Title", "Event", "Bot", "Session",
		"Referrer (sanitized)", "Referrer query params",
		"Original Referrer", "Browser", "Screen size", "Location",
		"Date"})

	var (
		last int64
		err  error
	)
	for {
		var hits Hits
		last, err = hits.List(ctx, 5000, last)
		if len(hits) == 0 {
			break
		}
		if err != nil {
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
			break
		}

		// Small amount of breathing space.
		if cfg.Prod {
			time.Sleep(500 * time.Millisecond)
		}
	}

	if err != nil {
		l.Error(err)
		_ = gzfp.Close()
		_ = fp.Close()
		_ = os.Remove(fp.Name())
		return
	}

	err = gzfp.Close()
	if err != nil {
		l.Error(err)
		return
	}
	err = fp.Sync() // Ensure stat is correct.
	if err != nil {
		l.Error(err)
		return
	}

	stat, err := fp.Stat()
	var size float64
	if err == nil {
		size = float64(stat.Size()) / 1024 / 1024
	}

	err = fp.Close()
	if err != nil {
		l.Error(err)
		return
	}

	err = os.Rename(fp.Name(), ExportFile(site))
	if err != nil {
		l.Error(err)
		return
	}

	user := GetUser(ctx)
	zmail.Send("GoatCounter export ready",
		mail.Address{Name: "GoatCounter export", Address: "support@goatcounter.com"},
		[]mail.Address{{Name: user.Name, Address: user.Email}},
		fmt.Sprintf(emailExportDone, site.URL(), size))
}

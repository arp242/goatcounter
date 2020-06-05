// Copyright © 2019 Martin Tournoij <martin@arp242.net>
// This file is part of GoatCounter and published under the terms of the EUPL
// v1.2, which can be found in the LICENSE file or at http://eupl12.zgo.at

package cron_test

import (
	"bytes"
	"context"
	"fmt"
	"testing"
	"time"

	"zgo.at/blackmail"
	"zgo.at/goatcounter"
	"zgo.at/goatcounter/cron"
	"zgo.at/goatcounter/gctest"
	"zgo.at/goatcounter/pack"
	"zgo.at/tz"
	"zgo.at/zhttp"
	"zgo.at/zlog"
)

func TestEmailReports(t *testing.T) {
	// Week from 17th to 23rd of June.
	n := time.Date(2019, 6, 18, 14, 42, 0, 0, time.UTC)
	d := time.Hour * 24
	goatcounter.Now = func() time.Time { return n }

	tests := []struct {
		name  string
		setup func(ctx context.Context) (context.Context, goatcounter.Site)
	}{
		{
			"no pages",
			func(ctx context.Context) (context.Context, goatcounter.Site) {
				return gctest.Site(ctx, t, goatcounter.Site{
					Settings: goatcounter.SiteSettings{
						EmailReports: goatcounter.EmailReportWeekly,
						Timezone:     tz.UTC,
					},
				})
			},
		},

		{
			"one page",
			func(ctx context.Context) (context.Context, goatcounter.Site) {
				ctx, site := gctest.Site(ctx, t, goatcounter.Site{
					CreatedAt: n.Add(-7 * d),
					Settings: goatcounter.SiteSettings{
						EmailReports: goatcounter.EmailReportWeekly,
						Timezone:     tz.UTC,
					},
				})

				gctest.StoreHits(ctx, t,
					goatcounter.Hit{Site: site.ID, Path: "/test", CreatedAt: n},
					goatcounter.Hit{Site: site.ID, Path: "/test", CreatedAt: n},
					goatcounter.Hit{Site: site.ID, Path: "/test", CreatedAt: n.Add(-6 * d)},
				)

				return ctx, site
			},
		},
	}

	zhttp.InitTpl(pack.Templates)
	zlog.Config.Debug = []string{"all"}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, clean := gctest.DB(t)
			defer clean()

			ctx, site := tt.setup(ctx)
			_ = site

			buf := new(bytes.Buffer)
			blackmail.DefaultMailer = blackmail.NewMailer(blackmail.ConnectWriter,
				blackmail.MailerOut(buf))

			err := cron.EmailReports(ctx)
			if err != nil {
				t.Fatal(err)
			}

			fmt.Println("BUF", buf.String(), "END")

			// if got != tt.want {
			// 	t.Errorf("\ngot:  %q\nwant: %q", got, tt.want)
			// }
			// if !reflect.DeepEqual(got, want) {
			// 	t.Errorf("\ngot:  %#v\nwant: %#v", tt.in, tt.want)
			// }
			// if d := ztest.Diff(got, tt.want) {
			// 	t.Errorf(d)
			// }
		})
	}
}

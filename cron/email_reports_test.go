package cron_test

import (
	"bytes"
	"context"
	"io/fs"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	"zgo.at/blackmail"
	"zgo.at/goatcounter/v2"
	"zgo.at/goatcounter/v2/cron"
	"zgo.at/goatcounter/v2/gctest"
	"zgo.at/tz"
	"zgo.at/zstd/zgo"
	"zgo.at/zstd/ztest"
	"zgo.at/zstd/ztime"
	"zgo.at/ztpl"
)

func TestEmailReports(t *testing.T) {
	files, _ := fs.Sub(os.DirFS(zgo.ModuleRoot()), "tpl")
	err := ztpl.Init(files)
	if err != nil {
		t.Fatal(err)
	}

	day := 24 * time.Hour
	now := time.Date(2019, 6, 17, 0, 1, 0, 0, time.UTC)
	ztime.Now = func() time.Time { return now }
	t.Cleanup(func() { ztime.Now = func() time.Time { return time.Now().UTC() } })

	tests := []struct {
		name  string
		setup func(ctx context.Context) context.Context
		want  string
	}{
		// No pages → don't send out anything.
		{
			"no pages",
			func(ctx context.Context) context.Context {
				return gctest.Site(ctx, t, nil, &goatcounter.User{
					LastReportAt: now.Add(-day),
					Settings: goatcounter.UserSettings{
						EmailReports: goatcounter.EmailReportDaily,
						Timezone:     tz.UTC,
					},
				})
			},
			"",
		},
		{
			"day",
			func(ctx context.Context) context.Context {
				ctx = gctest.Site(ctx, t, nil, &goatcounter.User{
					LastReportAt: now.Add(-day),
					Settings: goatcounter.UserSettings{
						EmailReports: goatcounter.EmailReportDaily,
						Timezone:     tz.UTC,
					},
				})
				sID := goatcounter.MustGetSite(ctx).ID
				gctest.StoreHits(ctx, t, false,
					goatcounter.Hit{Site: sID, FirstVisit: true, Path: "/a", CreatedAt: now.Add(-1 * time.Hour)},
					goatcounter.Hit{Site: sID, FirstVisit: true, Path: "/a", CreatedAt: now.Add(-48 * time.Hour)},

					goatcounter.Hit{Site: sID, FirstVisit: true, Path: "/b", CreatedAt: now.Add(-1 * time.Hour)},
					goatcounter.Hit{Site: sID, FirstVisit: true, Path: "/b", CreatedAt: now.Add(-1 * time.Hour)},
					goatcounter.Hit{Site: sID, FirstVisit: true, Path: "/b", CreatedAt: now.Add(-25 * time.Hour)},

					goatcounter.Hit{Site: sID, FirstVisit: true, Path: "/c", CreatedAt: now.Add(-1 * time.Hour)},
					goatcounter.Hit{Site: sID, FirstVisit: true, Path: "/c", CreatedAt: now.Add(-22 * time.Hour)},

					goatcounter.Hit{Site: sID, FirstVisit: true, Path: "/d", CreatedAt: now.Add(-1 * time.Hour), Ref: "xx"},
					goatcounter.Hit{Site: sID, FirstVisit: false, Path: "/d", CreatedAt: now.Add(-1 * time.Hour), Ref: "xx"},
					goatcounter.Hit{Site: sID, FirstVisit: true, Path: "/d", CreatedAt: now.Add(-25 * time.Hour), Ref: "xx"},
					goatcounter.Hit{Site: sID, FirstVisit: false, Path: "/d", CreatedAt: now.Add(-25 * time.Hour), Ref: "xx"},
				)
				return ctx
			}, `
				Path                                   Visitors   Growth
				/c                                            2    (new)
				/b                                            2     100%
				/d                                            1       0%
				/a                                            1    (new)
				Referrer                                        Visitors
				(no data)                                              5
				xx                                                     1
			`,
		},
		{
			"week",
			func(ctx context.Context) context.Context {
				ctx = gctest.Site(ctx, t, nil, &goatcounter.User{
					LastReportAt: now.Add(-23 * time.Hour),
					Settings: goatcounter.UserSettings{
						EmailReports: goatcounter.EmailReportWeekly,
						Timezone:     tz.UTC,
					},
				})
				sID := goatcounter.MustGetSite(ctx).ID
				gctest.StoreHits(ctx, t, false,
					goatcounter.Hit{Site: sID, FirstVisit: true, Path: "/a", CreatedAt: now.Add(-1 * time.Hour)},
					goatcounter.Hit{Site: sID, FirstVisit: true, Path: "/a", CreatedAt: now.Add(-48 * time.Hour)},

					goatcounter.Hit{Site: sID, FirstVisit: true, Path: "/b", CreatedAt: now.Add(-1 * time.Hour)},
					goatcounter.Hit{Site: sID, FirstVisit: true, Path: "/b", CreatedAt: now.Add(-1 * time.Hour)},
					goatcounter.Hit{Site: sID, FirstVisit: true, Path: "/b", CreatedAt: now.Add(-25 * time.Hour)},

					goatcounter.Hit{Site: sID, FirstVisit: true, Path: "/c", CreatedAt: now.Add(-1 * time.Hour)},
					goatcounter.Hit{Site: sID, FirstVisit: true, Path: "/c", CreatedAt: now.Add(-22 * time.Hour)},

					goatcounter.Hit{Site: sID, FirstVisit: true, Path: "/d", CreatedAt: now.Add(-1 * time.Hour), Ref: "xx"},
					goatcounter.Hit{Site: sID, FirstVisit: false, Path: "/d", CreatedAt: now.Add(-1 * time.Hour), Ref: "xx"},
					goatcounter.Hit{Site: sID, FirstVisit: true, Path: "/d", CreatedAt: now.Add(-25 * time.Hour), Ref: "xx"},
					goatcounter.Hit{Site: sID, FirstVisit: false, Path: "/d", CreatedAt: now.Add(-25 * time.Hour), Ref: "xx"},
				)
				return ctx
			}, `
				Path                                   Visitors   Growth
				/b                                            3    (new)
				/d                                            2    (new)
				/c                                            2    (new)
				/a                                            2    (new)
				Referrer                                        Visitors
				(no data)                                              7
				xx                                                     2
		 	`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := tt.setup(gctest.DB(t))
			goatcounter.Config(ctx).EmailFrom = "test@goatcounter.localhost.com"

			buf := new(bytes.Buffer)
			blackmail.DefaultMailer = blackmail.NewMailer(blackmail.ConnectWriter, blackmail.MailerOut(buf))

			err := cron.TaskEmailReports()
			if err != nil {
				t.Fatal(err)
			}
			cron.WaitEmailReports()

			if tt.want == "" {
				if buf.String() != "" {
					t.Errorf("sent out email:\n%s", buf.String())
				}
				return
			}

			// Compare a somewhat trimmed-down version of the text table.
			have := regexp.MustCompile(`(?s)Top 10 pages.*This is the text version`).FindString(buf.String())
			have = strings.ReplaceAll(have, "\r\n", "\n")
			have = strings.ReplaceAll(have, "This is the text version", "")
			have = strings.TrimSpace(have)
			have = strings.ReplaceAll(have, `Top 10 pages`, ``)
			have = strings.ReplaceAll(have, `Top 10 referrers`, ``)
			have = strings.ReplaceAll(have, `--------------------------------------------------------`, ``)
			have = regexp.MustCompile(`(?m)^\s+`).ReplaceAllString(have, "")

			tt.want = ztest.NormalizeIndent(tt.want)
			if d := ztest.Diff(have, tt.want); d != "" {
				t.Error(d + "\n\n" + have)
			}
		})
	}
}

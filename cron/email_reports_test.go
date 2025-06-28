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

	tests := []struct {
		name  string
		setup func(ctx context.Context) context.Context
		want  string
	}{
		{
			"no pages",
			func(ctx context.Context) context.Context {
				return gctest.Site(ctx, t, nil, &goatcounter.User{
					LastReportAt: ztime.Now(ctx).Add(-day),
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
					LastReportAt: ztime.Now(ctx).Add(-day),
					Settings: goatcounter.UserSettings{
						EmailReports: goatcounter.EmailReportDaily,
						Timezone:     tz.UTC,
					},
				})
				sID := goatcounter.MustGetSite(ctx).ID
				gctest.StoreHits(ctx, t, false,
					goatcounter.Hit{Site: sID, FirstVisit: true, Path: "/a", CreatedAt: ztime.Now(ctx).Add(-1 * time.Hour)},
					goatcounter.Hit{Site: sID, FirstVisit: true, Path: "/a", CreatedAt: ztime.Now(ctx).Add(-48 * time.Hour)},

					goatcounter.Hit{Site: sID, FirstVisit: true, Path: "/b", CreatedAt: ztime.Now(ctx).Add(-1 * time.Hour)},
					goatcounter.Hit{Site: sID, FirstVisit: true, Path: "/b", CreatedAt: ztime.Now(ctx).Add(-1 * time.Hour)},
					goatcounter.Hit{Site: sID, FirstVisit: true, Path: "/b", CreatedAt: ztime.Now(ctx).Add(-25 * time.Hour)},

					goatcounter.Hit{Site: sID, FirstVisit: true, Path: "/c", CreatedAt: ztime.Now(ctx).Add(-1 * time.Hour)},
					goatcounter.Hit{Site: sID, FirstVisit: true, Path: "/c", CreatedAt: ztime.Now(ctx).Add(-22 * time.Hour)},

					goatcounter.Hit{Site: sID, FirstVisit: true, Path: "/d", CreatedAt: ztime.Now(ctx).Add(-1 * time.Hour), Ref: "xx"},
					goatcounter.Hit{Site: sID, FirstVisit: false, Path: "/d", CreatedAt: ztime.Now(ctx).Add(-1 * time.Hour), Ref: "xx"},
					goatcounter.Hit{Site: sID, FirstVisit: true, Path: "/d", CreatedAt: ztime.Now(ctx).Add(-25 * time.Hour), Ref: "xx"},
					goatcounter.Hit{Site: sID, FirstVisit: false, Path: "/d", CreatedAt: ztime.Now(ctx).Add(-25 * time.Hour), Ref: "xx"},
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
					LastReportAt: ztime.Now(ctx).Add(-167 * time.Hour),
					Settings: goatcounter.UserSettings{
						EmailReports: goatcounter.EmailReportWeekly,
						Timezone:     tz.UTC,
					},
				})
				sID := goatcounter.MustGetSite(ctx).ID
				gctest.StoreHits(ctx, t, false,
					goatcounter.Hit{Site: sID, FirstVisit: true, Path: "/a", CreatedAt: ztime.Now(ctx).Add(-1 * time.Hour)},
					goatcounter.Hit{Site: sID, FirstVisit: true, Path: "/a", CreatedAt: ztime.Now(ctx).Add(-48 * time.Hour)},

					goatcounter.Hit{Site: sID, FirstVisit: true, Path: "/b", CreatedAt: ztime.Now(ctx).Add(-1 * time.Hour)},
					goatcounter.Hit{Site: sID, FirstVisit: true, Path: "/b", CreatedAt: ztime.Now(ctx).Add(-1 * time.Hour)},
					goatcounter.Hit{Site: sID, FirstVisit: true, Path: "/b", CreatedAt: ztime.Now(ctx).Add(-25 * time.Hour)},

					goatcounter.Hit{Site: sID, FirstVisit: true, Path: "/c", CreatedAt: ztime.Now(ctx).Add(-1 * time.Hour)},
					goatcounter.Hit{Site: sID, FirstVisit: true, Path: "/c", CreatedAt: ztime.Now(ctx).Add(-22 * time.Hour)},

					goatcounter.Hit{Site: sID, FirstVisit: true, Path: "/d", CreatedAt: ztime.Now(ctx).Add(-1 * time.Hour), Ref: "xx"},
					goatcounter.Hit{Site: sID, FirstVisit: false, Path: "/d", CreatedAt: ztime.Now(ctx).Add(-1 * time.Hour), Ref: "xx"},
					goatcounter.Hit{Site: sID, FirstVisit: true, Path: "/d", CreatedAt: ztime.Now(ctx).Add(-25 * time.Hour), Ref: "xx"},
					goatcounter.Hit{Site: sID, FirstVisit: false, Path: "/d", CreatedAt: ztime.Now(ctx).Add(-25 * time.Hour), Ref: "xx"},
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

		{
			"multiple sites",
			func(ctx context.Context) context.Context {
				ctx = gctest.Site(ctx, t, &goatcounter.Site{Code: "bb"}, &goatcounter.User{
					LastReportAt: ztime.Now(ctx).Add(-167 * time.Hour),
					Settings: goatcounter.UserSettings{
						EmailReports: goatcounter.EmailReportWeekly,
						Timezone:     tz.UTC,
					},
				})
				sID := goatcounter.MustGetSite(ctx).ID

				gctest.StoreHits(ctx, t, false,
					goatcounter.Hit{Site: sID, FirstVisit: true, Path: "/a", CreatedAt: ztime.Now(ctx).Add(-1 * time.Hour)},
					goatcounter.Hit{Site: sID, FirstVisit: true, Path: "/a", CreatedAt: ztime.Now(ctx).Add(-48 * time.Hour)},

					goatcounter.Hit{Site: sID, FirstVisit: true, Path: "/b", CreatedAt: ztime.Now(ctx).Add(-1 * time.Hour)},
					goatcounter.Hit{Site: sID, FirstVisit: true, Path: "/b", CreatedAt: ztime.Now(ctx).Add(-1 * time.Hour)},
					goatcounter.Hit{Site: sID, FirstVisit: true, Path: "/b", CreatedAt: ztime.Now(ctx).Add(-25 * time.Hour)},

					goatcounter.Hit{Site: sID, FirstVisit: true, Path: "/c", CreatedAt: ztime.Now(ctx).Add(-1 * time.Hour)},
					goatcounter.Hit{Site: sID, FirstVisit: true, Path: "/c", CreatedAt: ztime.Now(ctx).Add(-22 * time.Hour)},

					goatcounter.Hit{Site: sID, FirstVisit: true, Path: "/d", CreatedAt: ztime.Now(ctx).Add(-1 * time.Hour), Ref: "xx"},
					goatcounter.Hit{Site: sID, FirstVisit: false, Path: "/d", CreatedAt: ztime.Now(ctx).Add(-1 * time.Hour), Ref: "xx"},
					goatcounter.Hit{Site: sID, FirstVisit: true, Path: "/d", CreatedAt: ztime.Now(ctx).Add(-25 * time.Hour), Ref: "xx"},
					goatcounter.Hit{Site: sID, FirstVisit: false, Path: "/d", CreatedAt: ztime.Now(ctx).Add(-25 * time.Hour), Ref: "xx"},
				)

				site2 := gctest.Site(ctx, t, &goatcounter.Site{Code: "aa", Parent: &sID}, goatcounter.MustGetUser(ctx))
				sID = goatcounter.MustGetSite(site2).ID
				gctest.StoreHits(site2, t, false,
					goatcounter.Hit{Site: sID, FirstVisit: true, Path: "/2nd", CreatedAt: ztime.Now(site2).Add(-1 * time.Hour)},
					goatcounter.Hit{Site: sID, FirstVisit: true, Path: "/xxx", CreatedAt: ztime.Now(site2).Add(-48 * time.Hour), Ref: "yy"},
				)

				// No views for this, so don't include
				site3 := gctest.Site(ctx, t, &goatcounter.Site{Code: "cc", Parent: &sID}, goatcounter.MustGetUser(ctx))
				sID = goatcounter.MustGetSite(site3).ID
				gctest.StoreHits(site3, t, false,
					goatcounter.Hit{Site: sID, FirstVisit: true, Path: "/2nd", CreatedAt: ztime.Now(site3).Add(-1000 * time.Hour)},
					goatcounter.Hit{Site: sID, FirstVisit: true, Path: "/xxx", CreatedAt: ztime.Now(site3).Add(-4800 * time.Hour), Ref: "yy"},
				)
				return ctx
			}, `
				Path                                   Visitors   Growth
				/xxx                                          1    (new)
				/2nd                                          1    (new)
				Referrer                                        Visitors
				(no data)                                              1
				yy                                                     1
				https://bb.test
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
			ctx := tt.setup(ztime.WithNow(gctest.DB(t), time.Date(2019, 6, 17, 0, 1, 0, 0, time.UTC)))
			buf := new(bytes.Buffer)
			ctx = blackmail.With(ctx, blackmail.NewWriter(buf))
			goatcounter.Config(ctx).EmailFrom = "test@goatcounter.localhost.com"

			err := cron.EmailReports(ctx)
			if err != nil {
				t.Fatal(err)
			}

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
			have = strings.ReplaceAll(have, strings.Repeat("-", 56), ``)
			have = regexp.MustCompile(`(=3D|=)+`).ReplaceAllString(have, "")
			have = regexp.MustCompile(`(?m)^\s+`).ReplaceAllString(have, "")

			tt.want = ztest.NormalizeIndent(tt.want)
			if d := ztest.Diff(have, tt.want); d != "" {
				t.Error(d + "\n\n" + have)
			}
		})
	}
}

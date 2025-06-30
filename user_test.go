package goatcounter_test

import (
	"context"
	"testing"
	"time"

	"zgo.at/goatcounter/v2"
	"zgo.at/tz"
	"zgo.at/zstd/ztime"
)

func TestUserEmailReportRange(t *testing.T) {
	now := time.Date(2019, 6, 18, 14, 42, 0, 0, time.UTC)
	wita := tz.MustNew("", "Asia/Makassar")

	tests := []struct {
		user               goatcounter.User
		wantStart, wantEnd time.Time
	}{
		{goatcounter.User{
			LastReportAt: now,
			Settings: goatcounter.UserSettings{
				EmailReports: goatcounter.EmailReportDaily,
				Timezone:     tz.UTC,
			},
		}, ztime.FromString("2019-06-18 00:00:00"), ztime.FromString("2019-06-18 23:59:59")},
		{goatcounter.User{
			LastReportAt: now,
			Settings: goatcounter.UserSettings{
				SundayStartsWeek: false,
				EmailReports:     goatcounter.EmailReportWeekly,
				Timezone:         tz.UTC,
			},
		}, ztime.FromString("2019-06-17 00:00:00"), ztime.FromString("2019-06-23 23:59:59")},
		{goatcounter.User{
			LastReportAt: now,
			Settings: goatcounter.UserSettings{
				SundayStartsWeek: true,
				EmailReports:     goatcounter.EmailReportWeekly,
				Timezone:         tz.UTC,
			},
		}, ztime.FromString("2019-06-16 00:00:00"), ztime.FromString("2019-06-22 23:59:59")},
		{goatcounter.User{
			LastReportAt: now,
			Settings: goatcounter.UserSettings{
				SundayStartsWeek: false,
				EmailReports:     goatcounter.EmailReportBiWeekly,
				Timezone:         tz.UTC,
			},
		}, ztime.FromString("2019-06-17 00:00:00"), ztime.FromString("2019-06-30 23:59:59")},
		{goatcounter.User{
			LastReportAt: now,
			Settings: goatcounter.UserSettings{
				SundayStartsWeek: true,
				EmailReports:     goatcounter.EmailReportBiWeekly,
				Timezone:         tz.UTC,
			},
		}, ztime.FromString("2019-06-16 00:00:00"), ztime.FromString("2019-06-29 23:59:59")},

		{goatcounter.User{
			LastReportAt: now,
			Settings: goatcounter.UserSettings{
				EmailReports: goatcounter.EmailReportDaily,
				Timezone:     wita,
			},
		}, ztime.FromString("2019-06-17 16:00:00"), ztime.FromString("2019-06-18 15:59:59")},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			ctx := ztime.WithNow(context.Background(), now)
			rng := tt.user.EmailReportRange(ctx)
			if !rng.Start.Equal(tt.wantStart) {
				t.Errorf("start wrong\nwant: %s\nhave: %s\n", tt.wantStart, rng.Start)
			}
			if !rng.End.Equal(tt.wantEnd) {
				t.Errorf("end wrong\nwant: %s\nhave: %s\n", tt.wantEnd, rng.End)
			}
		})
	}
}

package handlers

import (
	"context"
	"net/http/httptest"
	"slices"
	"testing"
	"time"

	"zgo.at/goatcounter/v2"
	"zgo.at/zstd/ztime"
)

func TestDashboard(t *testing.T) {
	tests := []handlerTest{
		{
			name:     "no-data",
			router:   newBackend,
			auth:     true,
			wantCode: 200,
			wantBody: "<strong>No data received</strong>",
		},
	}

	for _, tt := range tests {
		runTest(t, tt, nil)
	}
}

func TestGetGroup(t *testing.T) {
	tests := []struct {
		days      int
		saved     goatcounter.Group
		query     string
		wantGroup goatcounter.Group
		wantAllow goatcounter.Groups
	}{
		{3, goatcounter.GroupHourly, "", goatcounter.GroupHourly,
			goatcounter.Groups{goatcounter.GroupHourly}},
		{3, goatcounter.GroupDaily, "", goatcounter.GroupHourly,
			goatcounter.Groups{goatcounter.GroupHourly}},

		{30, goatcounter.GroupHourly, "", goatcounter.GroupHourly,
			goatcounter.Groups{goatcounter.GroupHourly, goatcounter.GroupDaily}},
		{30, goatcounter.GroupDaily, "", goatcounter.GroupDaily,
			goatcounter.Groups{goatcounter.GroupHourly, goatcounter.GroupDaily}},
		{30, goatcounter.GroupWeekly, "", goatcounter.GroupHourly,
			goatcounter.Groups{goatcounter.GroupHourly, goatcounter.GroupDaily}},
		{30, goatcounter.GroupDaily, "hour", goatcounter.GroupHourly,
			goatcounter.Groups{goatcounter.GroupHourly, goatcounter.GroupDaily}},

		{120, goatcounter.GroupHourly, "", goatcounter.GroupDaily,
			goatcounter.Groups{goatcounter.GroupDaily, goatcounter.GroupWeekly}},
		{120, goatcounter.GroupWeekly, "", goatcounter.GroupWeekly,
			goatcounter.Groups{goatcounter.GroupDaily, goatcounter.GroupWeekly}},

		{365, goatcounter.GroupMonthly, "", goatcounter.GroupMonthly,
			goatcounter.Groups{goatcounter.GroupDaily, goatcounter.GroupWeekly, goatcounter.GroupMonthly}},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			start := ztime.FromString("2020-06-18")
			rng := ztime.NewRange(start).To(start.AddDate(0, 0, tt.days).Add(24*time.Hour - time.Second))

			r := httptest.NewRequest("GET", "/?group="+tt.query, nil)
			group, allow := getGroup(r, tt.saved, rng)
			if group != tt.wantGroup || !slices.Equal(allow, tt.wantAllow) {
				t.Errorf("\nhave: %s, %s\nwant: %s, %s", group, allow, tt.wantGroup, tt.wantAllow)
			}
		})
	}
}

func TestTimeRange(t *testing.T) {
	tests := []struct {
		rng, now, wantStart, wantEnd string
	}{
		{"week", "2020-12-02",
			"2020-11-25 00:00:00", "2020-12-02 23:59:59"},
		{"month", "2020-01-18",
			"2019-12-18 00:00:00", "2020-01-18 23:59:59"},
		{"quarter", "2020-01-18",
			"2019-10-18 00:00:00", "2020-01-18 23:59:59"},
		{"half-year", "2020-01-18",
			"2019-07-18 00:00:00", "2020-01-18 23:59:59"},
		{"year", "2020-01-18",
			"2019-01-18 00:00:00", "2020-01-18 23:59:59"},

		{"0", "2020-06-18",
			"2020-06-18 00:00:00", "2020-06-18 23:59:59"},
		{"1", "2020-06-18",
			"2020-06-17 00:00:00", "2020-06-18 23:59:59"},
		{"42", "2020-06-18",
			"2020-05-07 00:00:00", "2020-06-18 23:59:59"},
	}

	for _, tt := range tests {
		t.Run(tt.rng+"-"+tt.now, func(t *testing.T) {
			t.Run("UTC", func(t *testing.T) {
				ctx := ztime.WithNow(context.Background(), ztime.FromString(tt.now))
				rng := timeRange(ctx, tt.rng, time.UTC, false)
				gotStart := rng.Start.Format("2006-01-02 15:04:05")
				gotEnd := rng.End.Format("2006-01-02 15:04:05")

				if gotStart != tt.wantStart || gotEnd != tt.wantEnd {
					t.Errorf("\nhave: %q, %q\nwant: %q, %q",
						gotStart, gotEnd, tt.wantStart, tt.wantEnd)
				}
			})

			// t.Run("Asia/Makassar", func(t *testing.T) {
			// })
		})
	}
}

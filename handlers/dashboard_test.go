package handlers

import (
	"context"
	"testing"
	"time"

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

		// TODO: also test with sundayStartsWeek
		{"week-cur", "2020-01-01",
			"2019-12-30 00:00:00", "2020-01-05 23:59:59"},

		{"month-cur", "2020-01-01",
			"2020-01-01 00:00:00", "2020-01-31 23:59:59"},
		{"month-cur", "2020-01-31",
			"2020-01-01 00:00:00", "2020-01-31 23:59:59"},

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
					t.Errorf("\ngot:  %q, %q\nwant: %q, %q",
						gotStart, gotEnd, tt.wantStart, tt.wantEnd)
				}
			})

			// t.Run("Asia/Makassar", func(t *testing.T) {
			// })
		})
	}
}

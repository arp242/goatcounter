package cron_test

import (
	"testing"
	"time"

	"zgo.at/goatcounter/v2"
	"zgo.at/goatcounter/v2/gctest"
	"zgo.at/zstd/zjson"
	"zgo.at/zstd/ztest"
	"zgo.at/zstd/ztime"
)

func TestCampaignStats(t *testing.T) {
	ctx := gctest.DB(t)

	site := goatcounter.MustGetSite(ctx)
	now := time.Date(2019, 8, 31, 14, 42, 0, 0, time.UTC)

	gctest.StoreHits(ctx, t, false, []goatcounter.Hit{
		{Site: site.ID, CreatedAt: now, Query: "utm_campaign=one", FirstVisit: true},
		{Site: site.ID, CreatedAt: now, Query: "utm_campaign=one"},
		{Site: site.ID, CreatedAt: now, Query: "utm_campaign=two"},
		{Site: site.ID, CreatedAt: now, Query: "utm_campaign=three", FirstVisit: true},
	}...)

	var have goatcounter.HitStats
	err := have.ListCampaigns(ctx, ztime.NewRange(now).To(now), goatcounter.PathFilter{}, 10, 0)
	if err != nil {
		t.Fatal(err)
	}

	want := `{
		"more": false,
		"stats": [
			{"count": 1, "id": "1", "name": "one"},
			{"count": 1, "id": "3", "name": "three"}
		]
	}`
	if d := ztest.Diff(zjson.MustMarshalString(have), want, ztest.DiffJSON); d != "" {
		t.Error(d)
	}
}

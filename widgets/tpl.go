// Copyright © 2019 Martin Tournoij – This file is part of GoatCounter and
// published under the terms of a slightly modified EUPL v1.2 license, which can
// be found in the LICENSE file or at https://license.goatcounter.com

package widgets

import (
	"context"
	"time"

	"zgo.at/goatcounter"
)

func (w Refs) TemplateData(ctx context.Context, shared SharedData) (string, interface{}) {
	return "", nil
}
func (w Totals) TemplateData(ctx context.Context, shared SharedData) (string, interface{}) {
	return "", nil
}
func (w AllTotals) TemplateData(ctx context.Context, shared SharedData) (string, interface{}) {
	return "", nil
}
func (w Max) TemplateData(ctx context.Context, shared SharedData) (string, interface{}) {
	return "", nil
}

func (w Pages) TemplateData(ctx context.Context, shared SharedData) (string, interface{}) {
	return "_dashboard_pages.gohtml", struct {
		Context     context.Context
		Pages       goatcounter.HitStats
		Site        *goatcounter.Site
		PeriodStart time.Time
		PeriodEnd   time.Time
		Daily       bool
		ForcedDaily bool
		Offset      int
		Max         int

		TotalDisplay       int
		TotalUniqueDisplay int

		TotalHits       int
		TotalUniqueHits int
		MorePages       bool

		Refs     goatcounter.Stats
		ShowRefs string
	}{
		ctx, w.Pages, shared.Site, shared.Args.Start, shared.Args.End, shared.Args.Daily,
		shared.Args.ForcedDaily, 1, shared.Max, w.Display,
		w.UniqueDisplay, shared.Total, shared.TotalUnique,
		w.More, w.Refs, shared.Args.ShowRefs,
	}
}

func (w Totalpages) TemplateData(ctx context.Context, shared SharedData) (string, interface{}) {
	return "_dashboard_totals.gohtml", struct {
		Context         context.Context
		Site            *goatcounter.Site
		Page            goatcounter.HitStat
		Daily           bool
		Max             int
		TotalHits       int
		TotalUniqueHits int
	}{ctx, shared.Site, w.Total, shared.Args.Daily, w.Max, shared.Total,
		shared.TotalUnique}
}

func (w Toprefs) TemplateData(ctx context.Context, shared SharedData) (string, interface{}) {
	return "_dashboard_toprefs.gohtml", struct {
		Context         context.Context
		TotalUniqueHits int
		Stats           goatcounter.Stats
	}{ctx, shared.AllTotalUnique, w.TopRefs}
}

func (w Browsers) TemplateData(ctx context.Context, shared SharedData) (string, interface{}) {
	return "_dashboard_browsers.gohtml", struct {
		Context         context.Context
		TotalUniqueHits int
		Stats           goatcounter.Stats
	}{ctx, shared.AllTotalUnique, w.Browsers}
}

func (w Systems) TemplateData(ctx context.Context, shared SharedData) (string, interface{}) {
	return "_dashboard_systems.gohtml", struct {
		Context         context.Context
		TotalUniqueHits int
		Stats           goatcounter.Stats
	}{ctx, shared.AllTotalUnique, w.Systems}
}

func (w Sizes) TemplateData(ctx context.Context, shared SharedData) (string, interface{}) {
	return "_dashboard_sizes.gohtml", struct {
		Context         context.Context
		TotalUniqueHits int
		Stats           goatcounter.Stats
	}{ctx, shared.AllTotalUnique, w.SizeStat}
}

func (w Locations) TemplateData(ctx context.Context, shared SharedData) (string, interface{}) {
	return "_dashboard_locations.gohtml", struct {
		Context         context.Context
		TotalUniqueHits int
		Stats           goatcounter.Stats
	}{ctx, shared.AllTotalUnique, w.LocStat}
}

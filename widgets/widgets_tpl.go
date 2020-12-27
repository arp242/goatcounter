// Copyright © 2019 Martin Tournoij – This file is part of GoatCounter and
// published under the terms of a slightly modified EUPL v1.2 license, which can
// be found in the LICENSE file or at https://license.goatcounter.com

package widgets

import (
	"context"
	"time"

	"zgo.at/goatcounter"
	"zgo.at/zstd/zint"
)

func (w Refs) RenderHTML(ctx context.Context, shared SharedData) (string, interface{}) {
	return "", nil
}
func (w TotalCount) RenderHTML(ctx context.Context, shared SharedData) (string, interface{}) {
	return "", nil
}
func (w Max) RenderHTML(ctx context.Context, shared SharedData) (string, interface{}) {
	return "", nil
}

func (w Pages) RenderHTML(ctx context.Context, shared SharedData) (string, interface{}) {
	t := "_dashboard_pages.gohtml"
	if shared.Args.AsText {
		t = "_dashboard_pages_text.gohtml"
	}

	// Correct max for chunked data in text view.
	if shared.Args.AsText {
		w.Max = 0
		for _, p := range w.Pages {
			m, _ := goatcounter.ChunkStat(p.Stats)
			if m > w.Max {
				w.Max = m
			}
		}
	}

	return t, struct {
		Context     context.Context
		Err         error
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

		Total       int
		TotalUnique int
		MorePages   bool

		Refs     goatcounter.Stats
		ShowRefs string
	}{
		ctx, w.err, w.Pages, shared.Site, shared.Args.Start, shared.Args.End, shared.Args.Daily,
		shared.Args.ForcedDaily, 1, w.Max, w.Display,
		w.UniqueDisplay, shared.Total, shared.TotalUnique,
		w.More, w.Refs, shared.Args.ShowRefs,
	}
}

func isCol(ctx context.Context, flag zint.Bitflag16) bool {
	return goatcounter.MustGetSite(ctx).Settings.Collect.Has(flag)
}

func (w TotalPages) RenderHTML(ctx context.Context, shared SharedData) (string, interface{}) {
	return "_dashboard_totals.gohtml", struct {
		Context     context.Context
		Err         error
		Site        *goatcounter.Site
		Page        goatcounter.HitStat
		Daily       bool
		Max         int
		Total       int
		TotalUnique int
	}{ctx, w.err, shared.Site, w.Total, shared.Args.Daily, w.Max, shared.Total,
		shared.TotalUnique}
}

func (w TopRefs) RenderHTML(ctx context.Context, shared SharedData) (string, interface{}) {
	return "_dashboard_toprefs.gohtml", struct {
		Context     context.Context
		Err         error
		IsCollected bool
		TotalUnique int
		Stats       goatcounter.Stats
	}{ctx, w.err, isCol(ctx, goatcounter.CollectReferrer), shared.TotalUnique, w.TopRefs}
}

func (w Browsers) RenderHTML(ctx context.Context, shared SharedData) (string, interface{}) {
	return "_dashboard_browsers.gohtml", struct {
		Context        context.Context
		Err            error
		IsCollected    bool
		TotalUniqueUTC int
		Stats          goatcounter.Stats
	}{ctx, w.err, isCol(ctx, goatcounter.CollectUserAgent), shared.TotalUniqueUTC, w.Browsers}
}

func (w Systems) RenderHTML(ctx context.Context, shared SharedData) (string, interface{}) {
	return "_dashboard_systems.gohtml", struct {
		Context        context.Context
		Err            error
		IsCollected    bool
		TotalUniqueUTC int
		Stats          goatcounter.Stats
	}{ctx, w.err, isCol(ctx, goatcounter.CollectUserAgent), shared.TotalUniqueUTC, w.Systems}
}

func (w Sizes) RenderHTML(ctx context.Context, shared SharedData) (string, interface{}) {
	return "_dashboard_sizes.gohtml", struct {
		Context        context.Context
		Err            error
		IsCollected    bool
		TotalUniqueUTC int
		Stats          goatcounter.Stats
	}{ctx, w.err, isCol(ctx, goatcounter.CollectScreenSize), shared.TotalUniqueUTC, w.SizeStat}
}

func (w Locations) RenderHTML(ctx context.Context, shared SharedData) (string, interface{}) {
	return "_dashboard_locations.gohtml", struct {
		Context        context.Context
		Err            error
		IsCollected    bool
		TotalUniqueUTC int
		Stats          goatcounter.Stats
	}{ctx, w.err, isCol(ctx, goatcounter.CollectLocation), shared.TotalUniqueUTC, w.LocStat}
}

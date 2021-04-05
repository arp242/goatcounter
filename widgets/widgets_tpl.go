// Copyright © 2019 Martin Tournoij – This file is part of GoatCounter and
// published under the terms of a slightly modified EUPL v1.2 license, which can
// be found in the LICENSE file or at https://license.goatcounter.com

package widgets

import (
	"context"

	"zgo.at/goatcounter"
	"zgo.at/zstd/zint"
	"zgo.at/zstd/ztime"
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
	if w.Max == 0 {
		w.Max = 10
	}

	return t, struct {
		Context context.Context
		Site    *goatcounter.Site
		User    *goatcounter.User

		Err         error
		Pages       goatcounter.HitLists
		Period      ztime.Range
		Daily       bool
		ForcedDaily bool
		Offset      int
		Max         int

		TotalDisplay       int
		TotalUniqueDisplay int

		Total             int
		TotalUnique       int
		TotalEvents       int
		TotalEventsUnique int
		MorePages         bool

		Refs     goatcounter.HitStats
		ShowRefs string
	}{
		ctx, shared.Site, shared.User,
		w.err, w.Pages, shared.Args.Rng, shared.Args.Daily,
		shared.Args.ForcedDaily, 1, w.Max, w.Display,
		w.UniqueDisplay, shared.Total, shared.TotalUnique, shared.TotalEvents, shared.TotalEventsUnique,
		w.More, w.Refs, shared.Args.ShowRefs,
	}
}

func isCol(ctx context.Context, flag zint.Bitflag16) bool {
	return goatcounter.MustGetSite(ctx).Settings.Collect.Has(flag)
}

func (w TotalPages) RenderHTML(ctx context.Context, shared SharedData) (string, interface{}) {
	return "_dashboard_totals.gohtml", struct {
		Context context.Context
		Site    *goatcounter.Site
		User    *goatcounter.User

		Err               error
		Page              goatcounter.HitList
		Daily             bool
		Max               int
		Total             int
		TotalUnique       int
		TotalEvents       int
		TotalEventsUnique int
	}{ctx, shared.Site, shared.User,
		w.err, w.Total, shared.Args.Daily, w.Max, shared.Total,
		shared.TotalUnique, shared.TotalEvents, shared.TotalEventsUnique}
}

func (w TopRefs) RenderHTML(ctx context.Context, shared SharedData) (string, interface{}) {
	return "_dashboard_toprefs.gohtml", struct {
		Context     context.Context
		Err         error
		IsCollected bool
		TotalUnique int
		Stats       goatcounter.HitStats
	}{ctx, w.err, isCol(ctx, goatcounter.CollectReferrer), shared.TotalUnique, w.TopRefs}
}

func (w Browsers) RenderHTML(ctx context.Context, shared SharedData) (string, interface{}) {
	return "_dashboard_browsers.gohtml", struct {
		Context        context.Context
		Err            error
		IsCollected    bool
		TotalUniqueUTC int
		Stats          goatcounter.HitStats
	}{ctx, w.err, isCol(ctx, goatcounter.CollectUserAgent), shared.TotalUniqueUTC, w.Browsers}
}

func (w Systems) RenderHTML(ctx context.Context, shared SharedData) (string, interface{}) {
	return "_dashboard_systems.gohtml", struct {
		Context        context.Context
		Err            error
		IsCollected    bool
		TotalUniqueUTC int
		Stats          goatcounter.HitStats
	}{ctx, w.err, isCol(ctx, goatcounter.CollectUserAgent), shared.TotalUniqueUTC, w.Systems}
}

func (w Sizes) RenderHTML(ctx context.Context, shared SharedData) (string, interface{}) {
	return "_dashboard_sizes.gohtml", struct {
		Context        context.Context
		Err            error
		IsCollected    bool
		TotalUniqueUTC int
		Stats          goatcounter.HitStats
	}{ctx, w.err, isCol(ctx, goatcounter.CollectScreenSize), shared.TotalUniqueUTC, w.SizeStat}
}

func (w Locations) RenderHTML(ctx context.Context, shared SharedData) (string, interface{}) {
	return "_dashboard_locations.gohtml", struct {
		Context        context.Context
		Err            error
		IsCollected    bool
		TotalUniqueUTC int
		Stats          goatcounter.HitStats
	}{ctx, w.err, isCol(ctx, goatcounter.CollectLocation), shared.TotalUniqueUTC, w.LocStat}
}

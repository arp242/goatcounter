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

func (w TotalCount) RenderHTML(ctx context.Context, shared SharedData) (string, interface{}) {
	return "", nil
}
func (w Max) RenderHTML(ctx context.Context, shared SharedData) (string, interface{}) {
	return "", nil
}

func (w Pages) RenderHTML(ctx context.Context, shared SharedData) (string, interface{}) {
	if w.Ref != "" {
		return "_dashboard_pages_refs.gohtml", struct {
			Context context.Context
			Site    *goatcounter.Site
			User    *goatcounter.User
			ID      int
			Err     error

			Refs        goatcounter.HitStats
			CountUnique int
		}{ctx, shared.Site, shared.User, w.id, w.err,
			w.Refs, shared.TotalUnique}
	}

	t := "_dashboard_pages"
	if shared.Args.AsText {
		t += "_text"
	}
	if shared.RowsOnly {
		t += "_rows"
	}
	t += ".gohtml"

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

		ID          int
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
		w.id, w.err, w.Pages, shared.Args.Rng, shared.Args.Daily,
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
		ID      int
		Err     error

		Align             bool
		NoEvents          bool
		Page              goatcounter.HitList
		Daily             bool
		Max               int
		Total             int
		TotalUnique       int
		TotalEvents       int
		TotalEventsUnique int
	}{ctx, shared.Site, shared.User, w.id, w.err,
		w.Align, w.NoEvents,
		w.Total, shared.Args.Daily, w.Max, shared.Total, shared.TotalUnique, shared.TotalEvents, shared.TotalEventsUnique}
}

func (w TopRefs) RenderHTML(ctx context.Context, shared SharedData) (string, interface{}) {
	return "_dashboard_toprefs.gohtml", struct {
		Context     context.Context
		ID          int
		RowsOnly    bool
		Err         error
		IsCollected bool
		TotalUnique int
		Stats       goatcounter.HitStats
		Ref         string
	}{ctx, w.id, shared.RowsOnly, w.err, isCol(ctx, goatcounter.CollectReferrer),
		shared.TotalUnique, w.TopRefs, w.Ref}
}

func (w Browsers) RenderHTML(ctx context.Context, shared SharedData) (string, interface{}) {
	return "_dashboard_browsers.gohtml", struct {
		Context        context.Context
		ID             int
		RowsOnly       bool
		Err            error
		IsCollected    bool
		TotalUniqueUTC int
		Stats          goatcounter.HitStats
		Browser        string
	}{ctx, w.id, shared.RowsOnly, w.err, isCol(ctx, goatcounter.CollectUserAgent),
		shared.TotalUniqueUTC, w.Browsers, w.Browser}
}

func (w Systems) RenderHTML(ctx context.Context, shared SharedData) (string, interface{}) {
	return "_dashboard_systems.gohtml", struct {
		Context        context.Context
		ID             int
		RowsOnly       bool
		Err            error
		IsCollected    bool
		TotalUniqueUTC int
		Stats          goatcounter.HitStats
		System         string
	}{ctx, w.id, shared.RowsOnly, w.err, isCol(ctx, goatcounter.CollectUserAgent),
		shared.TotalUniqueUTC, w.Systems, w.System}
}

func (w Sizes) RenderHTML(ctx context.Context, shared SharedData) (string, interface{}) {
	return "_dashboard_sizes.gohtml", struct {
		Context        context.Context
		ID             int
		RowsOnly       bool
		Err            error
		IsCollected    bool
		TotalUniqueUTC int
		Stats          goatcounter.HitStats
	}{ctx, w.id, shared.RowsOnly, w.err, isCol(ctx, goatcounter.CollectScreenSize), shared.TotalUniqueUTC, w.SizeStat}
}

func (w Locations) RenderHTML(ctx context.Context, shared SharedData) (string, interface{}) {
	cname := ""
	if w.err == nil && w.Country != "" {
		var l goatcounter.Location
		err := l.ByCode(ctx, w.Country)
		if err != nil {
			w.err = err
		}
		cname = l.CountryName
	}

	return "_dashboard_locations.gohtml", struct {
		Context        context.Context
		ID             int
		RowsOnly       bool
		Err            error
		IsCollected    bool
		TotalUniqueUTC int
		Stats          goatcounter.HitStats
		Country        string
		CountryName    string
	}{ctx, w.id, shared.RowsOnly, w.err, isCol(ctx, goatcounter.CollectLocation),
		shared.TotalUniqueUTC, w.LocStat, w.Country, cname}
}

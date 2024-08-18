// Copyright © Martin Tournoij – This file is part of GoatCounter and published
// under the terms of a slightly modified EUPL v1.2 license, which can be found
// in the LICENSE file or at https://license.goatcounter.com

package widgets

import (
	"context"
	"html/template"

	"zgo.at/goatcounter/v2"
	"zgo.at/z18n"
)

type Locations struct {
	id     int
	loaded bool
	err    error
	html   template.HTML
	s      goatcounter.WidgetSettings

	Limit  int
	Detail string
	Stats  goatcounter.HitStats
}

func (w Locations) Name() string { return "locations" }
func (w Locations) Type() string { return "hchart" }
func (w Locations) Label(ctx context.Context) string {
	return z18n.T(ctx, "label/loc-stats|Location stats")
}
func (w *Locations) SetHTML(h template.HTML)             { w.html = h }
func (w Locations) HTML() template.HTML                  { return w.html }
func (w *Locations) SetErr(h error)                      { w.err = h }
func (w Locations) Err() error                           { return w.err }
func (w Locations) ID() int                              { return w.id }
func (w Locations) Settings() goatcounter.WidgetSettings { return w.s }

func (w *Locations) SetSettings(s goatcounter.WidgetSettings) {
	w.s = s
	if x := s["limit"].Value; x != nil {
		w.Limit = int(x.(float64))
	}
	if x := s["key"].Value; x != nil {
		w.Detail = x.(string)
	}
}

func (w *Locations) GetData(ctx context.Context, a Args) (more bool, err error) {
	if w.Detail != "" {
		err = w.Stats.ListLocation(ctx, w.Detail, a.Rng, a.PathFilter, w.Limit, a.Offset)
	} else {
		err = w.Stats.ListLocations(ctx, a.Rng, a.PathFilter, w.Limit, a.Offset)
	}
	w.loaded = true
	return w.Stats.More, err
}

func (w Locations) RenderHTML(ctx context.Context, shared SharedData) (string, any) {
	header := z18n.T(ctx, "header/locations|Locations")
	if w.err == nil && w.Detail != "" {
		var l goatcounter.Location
		err := l.ByCode(ctx, w.Detail)
		if err != nil {
			w.err = err
		}
		header = z18n.T(ctx, "header/locations-for|Locations for %(country)", l.CountryName)
	}

	return "_dashboard_hchart.gohtml", struct {
		Context      context.Context
		Base         string
		ID           int
		CanConfigure bool
		RowsOnly     bool
		HasSubMenu   bool
		Loaded       bool
		Err          error
		IsCollected  bool
		Header       string
		TotalUTC     int
		Stats        goatcounter.HitStats
		Detail       string
	}{ctx, goatcounter.Config(ctx).BasePath, w.id, true, shared.RowsOnly, w.Detail == "", w.loaded, w.err, isCol(ctx, goatcounter.CollectLocation),
		header, shared.TotalUTC, w.Stats, w.Detail}
}

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

type Systems struct {
	id     int
	loaded bool
	err    error
	html   template.HTML
	s      goatcounter.WidgetSettings

	Limit  int
	Detail string
	Stats  goatcounter.HitStats
}

func (w Systems) Name() string { return "systems" }
func (w Systems) Type() string { return "hchart" }
func (w Systems) Label(ctx context.Context) string {
	return z18n.T(ctx, "label/system-stats|System stats")
}
func (w *Systems) SetHTML(h template.HTML)             { w.html = h }
func (w Systems) HTML() template.HTML                  { return w.html }
func (w *Systems) SetErr(h error)                      { w.err = h }
func (w Systems) Err() error                           { return w.err }
func (w Systems) ID() int                              { return w.id }
func (w Systems) Settings() goatcounter.WidgetSettings { return w.s }

func (w *Systems) SetSettings(s goatcounter.WidgetSettings) {
	if x := s["limit"].Value; x != nil {
		w.Limit = int(x.(float64))
	}
	if x := s["key"].Value; x != nil {
		w.Detail = x.(string)
	}
	w.s = s
}

func (w *Systems) GetData(ctx context.Context, a Args) (more bool, err error) {
	if w.Detail != "" {
		err = w.Stats.ListSystem(ctx, w.Detail, a.Rng, a.PathFilter, w.Limit, a.Offset)
	} else {
		err = w.Stats.ListSystems(ctx, a.Rng, a.PathFilter, w.Limit, a.Offset)
	}
	w.loaded = true
	return w.Stats.More, err
}

func (w Systems) RenderHTML(ctx context.Context, shared SharedData) (string, any) {
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
	}{ctx, goatcounter.Config(ctx).BasePath, w.id, true, shared.RowsOnly, w.Detail == "", w.loaded, w.err,
		isCol(ctx, goatcounter.CollectUserAgent), z18n.T(ctx, "header/systems|Systems"),
		shared.TotalUTC, w.Stats, w.Detail}
}

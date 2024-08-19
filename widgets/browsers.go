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

type Browsers struct {
	id     int
	loaded bool
	err    error
	html   template.HTML
	s      goatcounter.WidgetSettings

	Limit  int
	Detail string
	Stats  goatcounter.HitStats
}

func (w Browsers) Name() string { return "browsers" }
func (w Browsers) Type() string { return "hchart" }
func (w Browsers) Label(ctx context.Context) string {
	return z18n.T(ctx, "label/browser-stats|Browser stats")
}
func (w *Browsers) SetHTML(h template.HTML)             { w.html = h }
func (w Browsers) HTML() template.HTML                  { return w.html }
func (w *Browsers) SetErr(h error)                      { w.err = h }
func (w Browsers) Err() error                           { return w.err }
func (w Browsers) ID() int                              { return w.id }
func (w Browsers) Settings() goatcounter.WidgetSettings { return w.s }

func (w *Browsers) SetSettings(s goatcounter.WidgetSettings) {
	if x := s["limit"].Value; x != nil {
		w.Limit = int(x.(float64))
	}
	if x := s["key"].Value; x != nil {
		w.Detail = x.(string)
	}
	w.s = s
}

func (w *Browsers) GetData(ctx context.Context, a Args) (more bool, err error) {
	if w.Detail != "" {
		err = w.Stats.ListBrowser(ctx, w.Detail, a.Rng, a.PathFilter, w.Limit, a.Offset)
	} else {
		err = w.Stats.ListBrowsers(ctx, a.Rng, a.PathFilter, w.Limit, a.Offset)
	}
	w.loaded = true
	return w.Stats.More, err
}

func (w Browsers) RenderHTML(ctx context.Context, shared SharedData) (string, any) {
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
		isCol(ctx, goatcounter.CollectUserAgent), z18n.T(ctx, "header/browsers|Browsers"),
		shared.TotalUTC, w.Stats, w.Detail}
}

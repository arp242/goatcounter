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

type Languages struct {
	id     int
	loaded bool
	err    error
	html   template.HTML
	s      goatcounter.WidgetSettings

	Limit int
	Stats goatcounter.HitStats
}

func (w Languages) Name() string { return "languages" }
func (w Languages) Type() string { return "hchart" }
func (w Languages) Label(ctx context.Context) string {
	return z18n.T(ctx, "label/language-stats|Language stats")
}
func (w *Languages) SetHTML(h template.HTML)             { w.html = h }
func (w Languages) HTML() template.HTML                  { return w.html }
func (w *Languages) SetErr(h error)                      { w.err = h }
func (w Languages) Err() error                           { return w.err }
func (w Languages) ID() int                              { return w.id }
func (w Languages) Settings() goatcounter.WidgetSettings { return w.s }

func (w *Languages) SetSettings(s goatcounter.WidgetSettings) {
	w.s = s
	if x := s["limit"].Value; x != nil {
		w.Limit = int(x.(float64))
	}
}

func (w *Languages) GetData(ctx context.Context, a Args) (more bool, err error) {
	err = w.Stats.ListLanguages(ctx, a.Rng, a.PathFilter, w.Limit, a.Offset)
	w.loaded = true
	return w.Stats.More, err
}

func (w Languages) RenderHTML(ctx context.Context, shared SharedData) (string, any) {
	header := z18n.T(ctx, "header/languages|Languages")

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
	}{ctx, goatcounter.Config(ctx).BasePath, w.id, true, shared.RowsOnly, false, w.loaded, w.err,
		isCol(ctx, goatcounter.CollectLanguage),
		header, shared.TotalUTC, w.Stats}
}

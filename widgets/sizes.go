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

type Sizes struct {
	id     int
	loaded bool
	err    error
	html   template.HTML
	s      goatcounter.WidgetSettings

	Limit  int
	Detail string
	Stats  goatcounter.HitStats
}

func (w Sizes) Name() string                         { return "sizes" }
func (w Sizes) Type() string                         { return "hchart" }
func (w Sizes) Label(ctx context.Context) string     { return z18n.T(ctx, "label/size-stats|Size stats") }
func (w *Sizes) SetHTML(h template.HTML)             { w.html = h }
func (w Sizes) HTML() template.HTML                  { return w.html }
func (w *Sizes) SetErr(h error)                      { w.err = h }
func (w Sizes) Err() error                           { return w.err }
func (w Sizes) ID() int                              { return w.id }
func (w Sizes) Settings() goatcounter.WidgetSettings { return w.s }

func (w *Sizes) SetSettings(s goatcounter.WidgetSettings) {
	if x := s["key"].Value; x != nil {
		w.Detail = x.(string)
	}
	w.s = s
}

func (w *Sizes) GetData(ctx context.Context, a Args) (more bool, err error) {
	if w.Detail != "" {
		err = w.Stats.ListSize(ctx, w.Detail, a.Rng, a.PathFilter, 6, a.Offset)
	} else {
		err = w.Stats.ListSizes(ctx, a.Rng, a.PathFilter)
	}
	w.loaded = true
	return w.Stats.More, err
}

func (w Sizes) RenderHTML(ctx context.Context, shared SharedData) (string, any) {
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
	}{ctx, goatcounter.Config(ctx).BasePath, w.id, false, shared.RowsOnly, w.Detail == "", w.loaded, w.err,
		isCol(ctx, goatcounter.CollectScreenSize), z18n.T(ctx, "header/sizes|Sizes"),
		shared.TotalUTC, w.Stats, w.Detail}
}

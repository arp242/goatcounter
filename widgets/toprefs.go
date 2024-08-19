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

type TopRefs struct {
	id     int
	loaded bool
	err    error
	html   template.HTML
	s      goatcounter.WidgetSettings

	Limit   int
	Ref     string
	TopRefs goatcounter.HitStats
}

func (w TopRefs) Name() string                         { return "toprefs" }
func (w TopRefs) Type() string                         { return "hchart" }
func (w TopRefs) Label(ctx context.Context) string     { return z18n.T(ctx, "label/topref|Top referrals") }
func (w *TopRefs) SetHTML(h template.HTML)             { w.html = h }
func (w TopRefs) HTML() template.HTML                  { return w.html }
func (w *TopRefs) SetErr(h error)                      { w.err = h }
func (w TopRefs) Err() error                           { return w.err }
func (w TopRefs) ID() int                              { return w.id }
func (w TopRefs) Settings() goatcounter.WidgetSettings { return w.s }

func (w *TopRefs) SetSettings(s goatcounter.WidgetSettings) {
	if x := s["limit"].Value; x != nil {
		w.Limit = int(x.(float64))
	}
	if x := s["key"].Value; x != nil {
		w.Ref = x.(string)
	}
	w.s = s
}

func (w *TopRefs) GetData(ctx context.Context, a Args) (more bool, err error) {
	if w.Ref != "" {
		err = w.TopRefs.ListTopRef(ctx, w.Ref, a.Rng, a.PathFilter, w.Limit, a.Offset)
	} else {
		err = w.TopRefs.ListTopRefs(ctx, a.Rng, a.PathFilter, w.Limit, a.Offset)
	}
	w.loaded = true
	return w.TopRefs.More, err
}

func (w TopRefs) RenderHTML(ctx context.Context, shared SharedData) (string, any) {
	return "_dashboard_toprefs.gohtml", struct {
		Context      context.Context
		Base         string
		ID           int
		CanConfigure bool
		RowsOnly     bool
		HasSubMenu   bool
		Loaded       bool
		Err          error
		IsCollected  bool
		Total        int
		Stats        goatcounter.HitStats
		Ref          string
	}{ctx, goatcounter.Config(ctx).BasePath, w.id, true, shared.RowsOnly, w.Ref == "", w.loaded, w.err,
		isCol(ctx, goatcounter.CollectReferrer), shared.Total, w.TopRefs, w.Ref}
}

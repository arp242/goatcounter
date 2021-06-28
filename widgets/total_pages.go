// Copyright © 2019 Martin Tournoij – This file is part of GoatCounter and
// published under the terms of a slightly modified EUPL v1.2 license, which can
// be found in the LICENSE file or at https://license.goatcounter.com

package widgets

import (
	"context"
	"html/template"

	"zgo.at/goatcounter"
	"zgo.at/z18n"
)

type TotalPages struct {
	id   int
	err  error
	html template.HTML
	s    goatcounter.WidgetSettings

	Align, NoEvents bool
	Max             int
	Total           goatcounter.HitList
}

func (w TotalPages) Name() string { return "totalpages" }
func (w TotalPages) Type() string { return "full-width" }
func (w TotalPages) Label(ctx context.Context) string {
	return z18n.T(ctx, "label/total-pageviews|Total site pageviews")
}
func (w *TotalPages) SetHTML(h template.HTML)             { w.html = h }
func (w TotalPages) HTML() template.HTML                  { return w.html }
func (w *TotalPages) SetErr(h error)                      { w.err = h }
func (w TotalPages) Err() error                           { return w.err }
func (w TotalPages) Settings() goatcounter.WidgetSettings { return w.s }

func (w *TotalPages) SetSettings(s goatcounter.WidgetSettings) {
	if x := s["align"].Value; x != nil {
		w.Align = x.(bool)
	}
	if x := s["no-events"].Value; x != nil {
		w.NoEvents = x.(bool)
	}
	w.s = s
}

func (w *TotalPages) GetData(ctx context.Context, a Args) (more bool, err error) {
	w.Max, err = w.Total.Totals(ctx, a.Rng, a.PathFilter, a.Daily, w.NoEvents)
	return false, err
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

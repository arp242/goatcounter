// Copyright © 2019 Martin Tournoij – This file is part of GoatCounter and
// published under the terms of a slightly modified EUPL v1.2 license, which can
// be found in the LICENSE file or at https://license.goatcounter.com

package widgets

import (
	"context"
	"html/template"

	"zgo.at/goatcounter/v2"
)

type Max struct {
	err  error
	html template.HTML
	s    goatcounter.WidgetSettings

	Max int
}

func (w Max) Name() string                                                 { return "max" }
func (w Max) Type() string                                                 { return "data-only" }
func (w Max) Label(ctx context.Context) string                             { return "" }
func (w *Max) SetHTML(h template.HTML)                                     {}
func (w Max) HTML() template.HTML                                          { return w.html }
func (w *Max) SetErr(h error)                                              { w.err = h }
func (w Max) Err() error                                                   { return w.err }
func (w Max) Settings() goatcounter.WidgetSettings                         { return w.s }
func (w *Max) SetSettings(s goatcounter.WidgetSettings)                    { w.s = s }
func (w Max) RenderHTML(context.Context, SharedData) (string, interface{}) { return "", nil }
func (w *Max) GetData(ctx context.Context, a Args) (more bool, err error) {
	w.Max, err = goatcounter.GetMax(ctx, a.Rng, a.PathFilter, a.Daily)
	return false, err
}

type TotalCount struct {
	goatcounter.TotalCount

	err  error
	html template.HTML
	s    goatcounter.WidgetSettings

	NoEvents bool
}

func (w TotalCount) Name() string                                                 { return "totalcount" }
func (w TotalCount) Type() string                                                 { return "data-only" }
func (w TotalCount) Label(ctx context.Context) string                             { return "" }
func (w *TotalCount) SetHTML(h template.HTML)                                     {}
func (w TotalCount) HTML() template.HTML                                          { return w.html }
func (w *TotalCount) SetErr(h error)                                              { w.err = h }
func (w TotalCount) Err() error                                                   { return w.err }
func (w TotalCount) Settings() goatcounter.WidgetSettings                         { return w.s }
func (w *TotalCount) SetSettings(s goatcounter.WidgetSettings)                    { w.s = s }
func (w TotalCount) RenderHTML(context.Context, SharedData) (string, interface{}) { return "", nil }

func (w *TotalCount) GetData(ctx context.Context, a Args) (more bool, err error) {
	w.TotalCount, err = goatcounter.GetTotalCount(ctx, a.Rng, a.PathFilter, w.NoEvents)
	return false, err
}

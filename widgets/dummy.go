// Copyright © Martin Tournoij – This file is part of GoatCounter and published
// under the terms of a slightly modified EUPL v1.2 license, which can be found
// in the LICENSE file or at https://license.goatcounter.com

package widgets

import (
	"context"
	"html/template"

	"zgo.at/goatcounter/v2"
)

type Dummy struct {
}

func (w Dummy) Name() string                                                 { return "dummy" }
func (w Dummy) Type() string                                                 { return "hchart" }
func (w Dummy) Label(ctx context.Context) string                             { return "" }
func (w *Dummy) SetHTML(h template.HTML)                                     {}
func (w Dummy) HTML() template.HTML                                          { return "" }
func (w *Dummy) SetErr(h error)                                              {}
func (w Dummy) Err() error                                                   { return nil }
func (w Dummy) ID() int                                                      { return 0 }
func (w Dummy) Settings() goatcounter.WidgetSettings                         { return goatcounter.WidgetSettings{} }
func (w *Dummy) SetSettings(s goatcounter.WidgetSettings)                    {}
func (w Dummy) RenderHTML(context.Context, SharedData) (string, interface{}) { return "", nil }
func (w *Dummy) GetData(ctx context.Context, a Args) (more bool, err error) {
	return false, nil
}

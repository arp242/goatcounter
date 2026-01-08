package widgets

import (
	"context"
	"html/template"

	"zgo.at/goatcounter/v2"
	"zgo.at/z18n"
)

type TotalPages struct {
	id     int
	loaded bool
	err    error
	html   template.HTML
	s      goatcounter.WidgetSettings

	Align, NoEvents bool
	Style           string
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
func (w TotalPages) ID() int                              { return w.id }
func (w TotalPages) Settings() goatcounter.WidgetSettings { return w.s }

func (w *TotalPages) SetSettings(s goatcounter.WidgetSettings) {
	if x := s["align"].Value; x != nil {
		w.Align = x.(bool)
	}
	if x := s["no-events"].Value; x != nil {
		w.NoEvents = x.(bool)
	}
	if x := s["style"].Value; x != nil {
		w.Style = x.(string)
	}
	w.s = s
}

func (w *TotalPages) GetData(ctx context.Context, a Args) (more bool, err error) {
	w.Max, err = w.Total.Totals(ctx, a.Rng, a.PathFilter, a.Group, w.NoEvents)
	w.loaded = true
	return false, err
}

func (w TotalPages) RenderHTML(ctx context.Context, shared SharedData) (string, any) {
	return "_dashboard_totals.gohtml", struct {
		Context context.Context
		Site    *goatcounter.Site
		User    *goatcounter.User
		ID      int
		Loaded  bool
		Err     error

		Align    bool
		NoEvents bool
		Page     goatcounter.HitList
		Group    goatcounter.Group
		Max      int

		Total       int
		TotalEvents int

		Style string
	}{ctx, shared.Site, shared.User, w.id, w.loaded, w.err,
		w.Align, w.NoEvents,
		w.Total, shared.Args.Group, w.Max,
		shared.Total, shared.TotalEvents,
		w.Style}
}

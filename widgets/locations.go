package widgets

import (
	"context"
	"html/template"

	"zgo.at/goatcounter/v2"
	"zgo.at/z18n"
	"zgo.at/zstd/ztime"
)

type Locations struct {
	id     int
	loaded bool
	err    error
	html   template.HTML
	s      goatcounter.WidgetSettings

	Limit         int
	Detail        string
	Stats         goatcounter.HitStats
	MostlyUnknown bool
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
		w.MostlyUnknown = !goatcounter.Config(ctx).GoatcounterCom && goatcounter.GetUser(ctx).ID > 0 &&
			len(w.Stats.Stats) > 0 && w.Stats.Stats[0].ID == "" &&
			ztime.StartOf(a.Rng.End, ztime.Day).Equal(ztime.StartOf(ztime.Now(ctx), ztime.Day))
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
		Context       context.Context
		Base          string
		Name          string
		ID            int
		CanConfigure  bool
		RowsOnly      bool
		HasSubMenu    bool
		Loaded        bool
		Err           error
		IsCollected   bool
		Header        string
		TotalUTC      int
		Stats         goatcounter.HitStats
		Detail        string
		MostlyUnknown bool
	}{ctx, goatcounter.Config(ctx).BasePath, w.Name(), w.id, true, shared.RowsOnly, w.Detail == "", w.loaded, w.err,
		isCol(ctx, goatcounter.CollectLocation), header, shared.TotalUTC, w.Stats, w.Detail, w.MostlyUnknown}
}

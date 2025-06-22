package widgets

import (
	"context"
	"html/template"

	"zgo.at/goatcounter/v2"
	"zgo.at/z18n"
	"zgo.at/zstd/zstrconv"
)

type Campaigns struct {
	id     int
	loaded bool
	err    error
	html   template.HTML
	s      goatcounter.WidgetSettings

	Limit    int
	Campaign goatcounter.CampaignID
	Stats    goatcounter.HitStats
}

func (w Campaigns) Name() string                         { return "campaigns" }
func (w Campaigns) Type() string                         { return "hchart" }
func (w Campaigns) Label(ctx context.Context) string     { return z18n.T(ctx, "label/campaigns|Campaigns") }
func (w *Campaigns) SetHTML(h template.HTML)             { w.html = h }
func (w Campaigns) HTML() template.HTML                  { return w.html }
func (w *Campaigns) SetErr(h error)                      { w.err = h }
func (w Campaigns) Err() error                           { return w.err }
func (w Campaigns) ID() int                              { return w.id }
func (w Campaigns) Settings() goatcounter.WidgetSettings { return w.s }

func (w *Campaigns) SetSettings(s goatcounter.WidgetSettings) {
	w.s = s
	if x := s["limit"].Value; x != nil {
		w.Limit = int(x.(float64))
	}
	if x := s["key"].Value; x != nil {
		w.Campaign, _ = zstrconv.ParseInt[goatcounter.CampaignID](x.(string), 10)
	}
}

func (w *Campaigns) GetData(ctx context.Context, a Args) (more bool, err error) {
	if w.Campaign > 0 {
		err = w.Stats.ListCampaign(ctx, w.Campaign, a.Rng, a.PathFilter, w.Limit, a.Offset)
	} else {
		err = w.Stats.ListCampaigns(ctx, a.Rng, a.PathFilter, w.Limit, a.Offset)
	}
	w.loaded = true
	return w.Stats.More, err
}

func (w Campaigns) RenderHTML(ctx context.Context, shared SharedData) (string, any) {
	//return "_dashboard_campaigns.gohtml", struct {
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
		Campaign     goatcounter.CampaignID
	}{ctx, goatcounter.Config(ctx).BasePath, w.id, true, shared.RowsOnly, w.Campaign == 0, w.loaded, w.err,
		isCol(ctx, goatcounter.CollectReferrer), w.Label(ctx),
		shared.TotalUTC, w.Stats, w.Campaign}
}

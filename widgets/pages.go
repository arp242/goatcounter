// Copyright © Martin Tournoij – This file is part of GoatCounter and published
// under the terms of a slightly modified EUPL v1.2 license, which can be found
// in the LICENSE file or at https://license.goatcounter.com

package widgets

import (
	"context"
	"html/template"
	"sync"

	"zgo.at/errors"
	"zgo.at/goatcounter/v2"
	"zgo.at/z18n"
	"zgo.at/zlog"
	"zgo.at/zstd/ztime"
)

type Pages struct {
	id     int
	loaded bool
	err    error
	html   template.HTML
	s      goatcounter.WidgetSettings

	Ref                    string
	Style                  string
	Limit, LimitRefs       int
	Display, UniqueDisplay int
	More                   bool
	Pages                  goatcounter.HitLists
	Refs                   goatcounter.HitStats
	Max                    int
	Exclude                []int64
}

func (w Pages) Name() string                         { return "pages" }
func (w Pages) Type() string                         { return "full-width" }
func (w Pages) Label(ctx context.Context) string     { return z18n.T(ctx, "label/paths|Paths overview") }
func (w *Pages) SetHTML(h template.HTML)             { w.html = h }
func (w Pages) HTML() template.HTML                  { return w.html }
func (w *Pages) SetErr(h error)                      { w.err = h }
func (w Pages) Err() error                           { return w.err }
func (w Pages) ID() int                              { return w.id }
func (w Pages) Settings() goatcounter.WidgetSettings { return w.s }

func (w *Pages) SetSettings(s goatcounter.WidgetSettings) {
	w.s = s
	if x := s["limit_pages"].Value; x != nil {
		w.Limit = int(x.(float64))
	}
	if x := s["limit_refs"].Value; x != nil {
		w.LimitRefs = int(x.(float64))
	}
	if x := s["key"].Value; x != nil {
		w.Ref = x.(string)
	}
	if x := s["style"].Value; x != nil {
		w.Style = x.(string)
	}
}

func (w *Pages) GetData(ctx context.Context, a Args) (bool, error) {
	if w.Ref != "" {
		err := w.Refs.ListRefsByPath(ctx, w.Ref, a.Rng, w.LimitRefs, a.Offset)
		return w.Refs.More, err
	}

	var (
		wg   sync.WaitGroup
		errs = errors.NewGroup(2)
	)
	if a.ShowRefs != "" {
		wg.Add(1)
		go func() {
			defer zlog.Recover()
			defer wg.Done()
			errs.Append(w.Refs.ListRefsByPath(ctx, a.ShowRefs, a.Rng, w.LimitRefs, a.Offset))
		}()
	}

	var err error
	w.Display, w.UniqueDisplay, w.More, err = w.Pages.List(ctx, a.Rng, a.PathFilter, w.Exclude, w.Limit, a.Daily)
	errs.Append(err)

	wg.Wait()
	w.loaded = true
	return w.More, errs.ErrorOrNil()
}

func (w Pages) RenderHTML(ctx context.Context, shared SharedData) (string, interface{}) {
	if w.Ref != "" {
		return "_dashboard_pages_refs.gohtml", struct {
			Context context.Context
			Site    *goatcounter.Site
			User    *goatcounter.User
			ID      int
			Loaded  bool
			Err     error

			Refs        goatcounter.HitStats
			CountUnique int
		}{ctx, shared.Site, shared.User, w.id, w.loaded, w.err,
			w.Refs, shared.TotalUnique}
	}

	t := "_dashboard_pages"
	if w.Style == "text" {
		t += "_text"
	}
	if shared.RowsOnly {
		t += "_rows"
	}
	t += ".gohtml"

	// Correct max for chunked data in text view.
	if w.Style == "text" {
		w.Max = 0
		for _, p := range w.Pages {
			m, _ := goatcounter.ChunkStat(p.Stats)
			if m > w.Max {
				w.Max = m
			}
		}
	} else if len(w.Pages) > 0 && len(w.Pages[0].Stats) > 0 {
		// Set days in the future to -1; we filter this in the JS when rendering
		// the chart.
		// It's easier to do this here because JavaScript Date() has piss-poor
		// support for timezones.
		//
		// Only remove them if the last day is today: for everything else we
		// want to display the future as "greyed out".
		var (
			now   = ztime.Now().In(goatcounter.MustGetUser(ctx).Settings.Timezone.Loc())
			today = now.Format("2006-01-02")
			hour  = now.Hour()
		)
		if w.Pages[0].Stats[len(w.Pages[0].Stats)-1].Day == today {
			for i := range w.Pages {
				j := len(w.Pages[i].Stats) - 1
				w.Pages[i].Stats[j].Hourly = w.Pages[i].Stats[j].Hourly[:hour+1]
				w.Pages[i].Stats[j].HourlyUnique = w.Pages[i].Stats[j].HourlyUnique[:hour+1]
			}
		}
	}
	if w.Max == 0 {
		w.Max = 10
	}

	return t, struct {
		Context context.Context
		Site    *goatcounter.Site
		User    *goatcounter.User

		ID          int
		Loaded      bool
		Err         error
		Pages       goatcounter.HitLists
		Period      ztime.Range
		Daily       bool
		ForcedDaily bool
		Offset      int
		Max         int

		TotalDisplay       int
		TotalUniqueDisplay int

		Total             int
		TotalUnique       int
		TotalEvents       int
		TotalEventsUnique int
		MorePages         bool

		Style    string
		Refs     goatcounter.HitStats
		ShowRefs string
	}{
		ctx, shared.Site, shared.User,
		w.id, w.loaded, w.err, w.Pages, shared.Args.Rng, shared.Args.Daily,
		shared.Args.ForcedDaily, 1, w.Max, w.Display,
		w.UniqueDisplay, shared.Total, shared.TotalUnique, shared.TotalEvents, shared.TotalEventsUnique,
		w.More, w.Style, w.Refs, shared.Args.ShowRefs,
	}
}

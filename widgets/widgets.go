// Copyright © 2019 Martin Tournoij – This file is part of GoatCounter and
// published under the terms of a slightly modified EUPL v1.2 license, which can
// be found in the LICENSE file or at https://license.goatcounter.com

package widgets

import (
	"context"
	"fmt"
	"html/template"
	"sync"

	"zgo.at/errors"
	"zgo.at/goatcounter"
	"zgo.at/zlog"
	"zgo.at/zstd/zint"
	"zgo.at/zstd/ztime"
)

type (
	Widget interface {
		GetData(context.Context, Args) (bool, error)
		RenderHTML(context.Context, SharedData) (string, interface{})

		SetHTML(template.HTML)
		HTML() template.HTML
		SetErr(error)
		Err() error
		SetSettings(goatcounter.WidgetSettings)
		Settings() goatcounter.WidgetSettings

		Name() string
		Type() string // "full-width", "hchart"
		Label() string
	}

	Args struct {
		Rng         ztime.Range
		Offset      int
		PathFilter  []int64
		Daily       bool
		ForcedDaily bool
		AsText      bool
		ShowRefs    string
	}

	// SharedData gets passed to every widget.
	SharedData struct {
		Site *goatcounter.Site
		User *goatcounter.User
		Args Args

		RowsOnly          bool
		Total             int
		TotalUnique       int
		TotalUniqueUTC    int
		TotalEvents       int
		TotalEventsUnique int
	}
)

type List []Widget

var (
	FilterInternal zint.Bitflag8 = 0b0001
)

func FromSiteWidget(w goatcounter.Widget) Widget {
	ww := NewWidget(w.Name(), 0)
	ww.SetSettings(w.GetSettings())

	return ww
}

func FromSiteWidgets(www goatcounter.Widgets, params zint.Bitflag8) List {
	widgetList := make(List, 0, len(www)+4)
	if !params.Has(FilterInternal) {
		// We always need these to know the total number of pageviews.
		widgetList = append(widgetList, NewWidget("totalcount", 0))
	}
	for i, w := range www {
		ww := NewWidget(w.Name(), i)
		ww.SetSettings(w.GetSettings())

		switch w.Name() {
		case "pages":
			if !params.Has(FilterInternal) {
				widgetList = append(widgetList, NewWidget("max", 0))
			}
			// XXX
			// if params.Has(ShowRefs) {
			// 	r := NewWidget("refs", 0).(*Refs)
			// 	r.Limit = int(w.GetSetting("limit_refs").(float64))
			// 	widgetList = append(widgetList, r)
			// }
		}
		widgetList = append(widgetList, ww)
	}

	return widgetList
}

// Get a widget from the list by name.
func (l List) Get(name string) Widget {
	for _, w := range l {
		if w.Name() == name {
			return w
		}
	}
	return nil
}

// ListAllWidgets returns a static list of all widgets that this user can add.
func ListAllWidgets() List {
	return List{
		NewWidget("browsers", 0),
		NewWidget("locations", 0),
		NewWidget("pages", 0),
		NewWidget("sizes", 0),
		NewWidget("systems", 0),
		NewWidget("toprefs", 0),
		NewWidget("totalpages", 0),
	}
}

func NewWidget(name string, id int) Widget {
	switch name {
	case "totalcount":
		return &TotalCount{}
	case "max":
		return &Max{}

	case "pages":
		return &Pages{id: id}
	case "totalpages":
		return &TotalPages{id: id}
	case "toprefs":
		return &TopRefs{id: id}
	case "browsers":
		return &Browsers{id: id}
	case "systems":
		return &Systems{id: id}
	case "sizes":
		return &Sizes{id: id}
	case "locations":
		return &Locations{id: id}
	}
	panic(fmt.Errorf("unknown widget: %q", name))
}

func (w *TotalCount) GetData(ctx context.Context, a Args) (more bool, err error) {
	w.TotalCount, err = goatcounter.GetTotalCount(ctx, a.Rng, a.PathFilter, w.NoEvents)
	return false, err
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
	return w.More, errs.ErrorOrNil()
}
func (w *Max) GetData(ctx context.Context, a Args) (more bool, err error) {
	w.Max, err = goatcounter.GetMax(ctx, a.Rng, a.PathFilter, a.Daily)
	return false, err
}
func (w *TotalPages) GetData(ctx context.Context, a Args) (more bool, err error) {
	w.Max, err = w.Total.Totals(ctx, a.Rng, a.PathFilter, a.Daily, w.NoEvents)
	return false, err
}
func (w *TopRefs) GetData(ctx context.Context, a Args) (more bool, err error) {
	if w.Ref != "" {
		err = w.TopRefs.ListTopRef(ctx, w.Ref, a.Rng, a.PathFilter, w.Limit, a.Offset)
	} else {
		err = w.TopRefs.ListTopRefs(ctx, a.Rng, a.PathFilter, w.Limit, a.Offset)
	}
	return w.TopRefs.More, err
}
func (w *Browsers) GetData(ctx context.Context, a Args) (more bool, err error) {
	if w.Browser != "" {
		err = w.Browsers.ListBrowser(ctx, w.Browser, a.Rng, a.PathFilter, w.Limit, a.Offset)
	} else {
		err = w.Browsers.ListBrowsers(ctx, a.Rng, a.PathFilter, w.Limit, a.Offset)
	}
	return w.Browsers.More, err
}
func (w *Systems) GetData(ctx context.Context, a Args) (more bool, err error) {
	if w.System != "" {
		err = w.Systems.ListSystem(ctx, w.System, a.Rng, a.PathFilter, w.Limit, a.Offset)
	} else {
		err = w.Systems.ListSystems(ctx, a.Rng, a.PathFilter, w.Limit, a.Offset)
	}
	return w.Systems.More, err
}
func (w *Sizes) GetData(ctx context.Context, a Args) (more bool, err error) {
	if w.Size != "" {
		err = w.SizeStat.ListSize(ctx, w.Size, a.Rng, a.PathFilter, 6, a.Offset)
	} else {
		err = w.SizeStat.ListSizes(ctx, a.Rng, a.PathFilter)
	}
	return w.SizeStat.More, err
}
func (w *Locations) GetData(ctx context.Context, a Args) (more bool, err error) {
	if w.Country != "" {
		err = w.LocStat.ListLocation(ctx, w.Country, a.Rng, a.PathFilter, w.Limit, a.Offset)
	} else {
		err = w.LocStat.ListLocations(ctx, a.Rng, a.PathFilter, w.Limit, a.Offset)
	}
	return w.LocStat.More, err
}

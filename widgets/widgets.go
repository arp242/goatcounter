// Copyright © 2019 Martin Tournoij – This file is part of GoatCounter and
// published under the terms of a slightly modified EUPL v1.2 license, which can
// be found in the LICENSE file or at https://license.goatcounter.com

package widgets

import (
	"context"
	"fmt"
	"html/template"

	"zgo.at/goatcounter/v2"
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
		Label(context.Context) string
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

func FromSiteWidget(ctx context.Context, w goatcounter.Widget) Widget {
	ww := NewWidget(w.Name(), 0)
	ww.SetSettings(w.GetSettings(ctx))

	return ww
}

func FromSiteWidgets(ctx context.Context, www goatcounter.Widgets, params zint.Bitflag8) List {
	widgetList := make(List, 0, len(www)+4)
	if !params.Has(FilterInternal) {
		// We always need these to know the total number of pageviews.
		widgetList = append(widgetList, NewWidget("totalcount", 0))
	}
	for i, w := range www {
		ww := NewWidget(w.Name(), i)
		ww.SetSettings(w.GetSettings(ctx))

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

func isCol(ctx context.Context, flag zint.Bitflag16) bool {
	return goatcounter.MustGetSite(ctx).Settings.Collect.Has(flag)
}

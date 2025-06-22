package widgets

import (
	"context"
	"html/template"

	"zgo.at/goatcounter/v2"
	"zgo.at/goatcounter/v2/pkg/log"
	"zgo.at/zstd/zint"
	"zgo.at/zstd/ztime"
)

type (
	Widget interface {
		GetData(context.Context, Args) (bool, error)
		RenderHTML(context.Context, SharedData) (string, any)

		SetHTML(template.HTML)
		HTML() template.HTML
		SetErr(error)
		Err() error
		SetSettings(goatcounter.WidgetSettings)
		Settings() goatcounter.WidgetSettings
		ID() int

		Name() string
		Type() string // "full-width", "hchart"
		Label(context.Context) string
	}

	Args struct {
		Rng         ztime.Range
		Offset      int
		PathFilter  []goatcounter.PathID
		Daily       bool
		ForcedDaily bool
		ShowRefs    goatcounter.PathID
	}

	// SharedData gets passed to every widget.
	SharedData struct {
		Site *goatcounter.Site
		User *goatcounter.User
		Args Args

		RowsOnly    bool
		Total       int
		TotalUTC    int
		TotalEvents int
	}
)

type List []Widget

var (
	FilterInternal zint.Bitflag8 = 0b0001
)

func FromSiteWidget(ctx context.Context, w goatcounter.Widget) Widget {
	ww := NewWidget(ctx, w.Name(), 0)
	ww.SetSettings(w.GetSettings(ctx))

	return ww
}

func FromSiteWidgets(ctx context.Context, www goatcounter.Widgets, params zint.Bitflag8) List {
	widgetList := make(List, 0, len(www)+4)
	if !params.Has(FilterInternal) {
		// We always need these to know the total number of pageviews.
		widgetList = append(widgetList, NewWidget(ctx, "totalcount", 0))
	}
	for i, w := range www {
		ww := NewWidget(ctx, w.Name(), i)
		ww.SetSettings(w.GetSettings(ctx))
		widgetList = append(widgetList, ww)
	}

	return widgetList
}

// GetOne gets the first widget in the list by name.
//
// You usually want to use Get()! Only intended to get "internal" widgets where
// you know it will always have exactly one in the list.
func (l List) GetOne(name string) Widget {
	for _, w := range l {
		if w.Name() == name {
			return w
		}
	}
	return nil
}

// Get all widgets from the list by name.
func (l List) Get(name string) List {
	list := make([]Widget, 0, 1)
	for _, w := range l {
		if w.Name() == name {
			list = append(list, w)
		}
	}
	return list
}

// Initial gets all widgets that should be loaded on the initial pageview (all
// internal widgets + the first one).
func (l List) InitialAndLazy() (initial List, lazy List) {
	first := true
	initial = make(List, 0, 3)
	lazy = make(List, 0, max(len(l), len(l)-3))
	for _, w := range l {
		switch w.Name() {
		case "totalcount":
			initial = append(initial, w)
		default:
			if first {
				initial = append(initial, w)
				first = false
			} else {
				lazy = append(lazy, w)
			}
		}
	}
	return initial, lazy
}

// ListAllWidgets returns a static list of all widgets that this user can add.
func ListAllWidgets() List {
	return List{
		NewWidget(context.Background(), "browsers", 0),
		NewWidget(context.Background(), "locations", 0),
		NewWidget(context.Background(), "languages", 0),
		NewWidget(context.Background(), "pages", 0),
		NewWidget(context.Background(), "sizes", 0),
		NewWidget(context.Background(), "systems", 0),
		NewWidget(context.Background(), "toprefs", 0),
		NewWidget(context.Background(), "campaigns", 0),
		NewWidget(context.Background(), "totalpages", 0),
	}
}

func NewWidget(ctx context.Context, name string, id int) Widget {
	switch name {
	case "totalcount":
		return &TotalCount{}

	case "pages":
		return &Pages{id: id}
	case "totalpages":
		return &TotalPages{id: id}
	case "toprefs":
		return &TopRefs{id: id}
	case "campaigns":
		return &Campaigns{id: id}
	case "browsers":
		return &Browsers{id: id}
	case "systems":
		return &Systems{id: id}
	case "sizes":
		return &Sizes{id: id}
	case "locations":
		return &Locations{id: id}
	case "languages":
		return &Languages{id: id}
	}
	log.Errorf(ctx, "unknown widget: %q", name)
	return &Dummy{}
}

func isCol(ctx context.Context, flag zint.Bitflag16) bool {
	return goatcounter.MustGetSite(ctx).Settings.Collect.Has(flag)
}

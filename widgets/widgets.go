// Copyright © 2019 Martin Tournoij – This file is part of GoatCounter and
// published under the terms of a slightly modified EUPL v1.2 license, which can
// be found in the LICENSE file or at https://license.goatcounter.com

package widgets

import (
	"context"
	"fmt"
	"html/template"
	"time"

	"zgo.at/goatcounter"
)

type (
	Widget interface {
		GetData(context.Context, Args) error
		TemplateData(context.Context, SharedData) (string, interface{})
		SetHTML(template.HTML)
		Clone() Widget

		Name() string
		Type() string // "full-width", "hchart"
		HTML() template.HTML
	}

	Args struct {
		Start, End  time.Time
		PathFilter  []int64
		Daily       bool
		ForcedDaily bool
		ShowRefs    string
		AsText      bool
	}

	SharedData struct {
		Site *goatcounter.Site
		Args Args

		Total             int
		TotalUnique       int
		AllTotalUniqueUTC int
		Max               int
		Refs              goatcounter.Stats
	}
)

type List []Widget

func NewList(want []string) (List, error) {
	list := make(List, len(want))
	for i := range want {
		wid, err := New(want[i])
		if err != nil {
			return nil, err
		}
		list[i] = wid
	}
	return list, nil
}

func (l List) Get(name string) Widget {
	// For a short list using a loop is usually faster.
	for _, w := range l {
		if w.Name() == name {
			return w
		}
	}
	return nil
}

func (l List) Totals() (total, unique, allUnique, max int) {
	for _, w := range l {
		if w.Name() == "totals" {
			ww := w.(*Totals)
			total, unique = ww.Total, ww.TotalUnique
		}
		if w.Name() == "alltotals" {
			ww := w.(*AllTotals)
			allUnique = ww.AllTotalUniqueUTC
		}
		if w.Name() == "max" {
			ww := w.(*Max)
			max = ww.Max
		}
	}
	return
}

func (l List) Refs() goatcounter.Stats {
	for _, w := range l {
		if w.Name() == "refs" {
			ww := w.(*Refs)
			return ww.Refs
		}
	}
	panic("should never happen")
}

func New(name string) (Widget, error) {
	w, ok := list[name]
	if ok {
		return w.Clone(), nil
	}
	return nil, fmt.Errorf("unknown widget: %q", name)
}

func (w *Totals) GetData(ctx context.Context, a Args) (err error) {
	w.Total, w.TotalUnique, err = goatcounter.GetTotalCount(ctx, a.Start, a.End, a.PathFilter)
	return err
}

func (w *AllTotals) GetData(ctx context.Context, a Args) (err error) {
	_, w.AllTotalUniqueUTC, err = goatcounter.GetTotalCountUTC(ctx, a.Start, a.End, a.PathFilter)
	return err
}

func (w *Pages) GetData(ctx context.Context, a Args) (err error) {
	w.Display, w.UniqueDisplay, w.More, err = w.Pages.List(
		ctx, a.Start, a.End, a.PathFilter, nil, a.Daily)
	return err
}

func (w *Max) GetData(ctx context.Context, a Args) (err error) {
	w.Max, err = goatcounter.GetMax(ctx, a.Start, a.End, a.PathFilter, a.Daily)
	return err
}

func (w *Totalpages) GetData(ctx context.Context, a Args) (err error) {
	w.Max, err = w.Total.Totals(ctx, a.Start, a.End, a.PathFilter, a.Daily)
	return err
}

func (w *Refs) GetData(ctx context.Context, a Args) (err error) {
	return w.Refs.ListRefsByPath(ctx, a.ShowRefs, a.Start, a.End, 0)
}
func (w *Toprefs) GetData(ctx context.Context, a Args) (err error) {
	return w.TopRefs.ListTopRefs(ctx, a.Start, a.End, a.PathFilter, 0)
}
func (w *Browsers) GetData(ctx context.Context, a Args) (err error) {
	return w.Browsers.ListBrowsers(ctx, a.Start, a.End, a.PathFilter, 6, 0)
}
func (w *Systems) GetData(ctx context.Context, a Args) (err error) {
	return w.Systems.ListSystems(ctx, a.Start, a.End, a.PathFilter, 6, 0)
}
func (w *Sizes) GetData(ctx context.Context, a Args) (err error) {
	return w.SizeStat.ListSizes(ctx, a.Start, a.End, a.PathFilter)
}
func (w *Locations) GetData(ctx context.Context, a Args) (err error) {
	return w.LocStat.ListLocations(ctx, a.Start, a.End, a.PathFilter, 6, 0)
}

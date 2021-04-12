// Copyright © 2019 Martin Tournoij – This file is part of GoatCounter and
// published under the terms of a slightly modified EUPL v1.2 license, which can
// be found in the LICENSE file or at https://license.goatcounter.com

package widgets

import (
	"html/template"

	"zgo.at/goatcounter"
)

// Unselectable "internal" widgets.
type (
	TotalCount struct {
		goatcounter.TotalCount

		err  error
		html template.HTML
		s    goatcounter.WidgetSettings

		NoEvents bool
	}
	Max struct {
		err  error
		html template.HTML
		s    goatcounter.WidgetSettings

		Max int
	}
)

// Selectable widgets.
type (
	Pages struct {
		id   int
		err  error
		html template.HTML
		s    goatcounter.WidgetSettings

		Ref                    string
		Limit, LimitRefs       int
		Display, UniqueDisplay int
		More                   bool
		Pages                  goatcounter.HitLists
		Refs                   goatcounter.HitStats
		Max                    int
		Exclude                []int64
	}
	TotalPages struct {
		id   int
		err  error
		html template.HTML
		s    goatcounter.WidgetSettings

		Align, NoEvents bool
		Max             int
		Total           goatcounter.HitList
	}
	Refs struct {
		err  error
		html template.HTML
		s    goatcounter.WidgetSettings

		Limit int
		Refs  goatcounter.HitStats
	}
	TopRefs struct {
		id   int
		err  error
		html template.HTML
		s    goatcounter.WidgetSettings

		Limit   int
		Ref     string
		TopRefs goatcounter.HitStats
	}
	Browsers struct {
		id   int
		err  error
		html template.HTML
		s    goatcounter.WidgetSettings

		Limit    int
		Browser  string
		Browsers goatcounter.HitStats
	}
	Systems struct {
		id   int
		err  error
		html template.HTML
		s    goatcounter.WidgetSettings

		Limit   int
		System  string
		Systems goatcounter.HitStats
	}
	Sizes struct {
		id   int
		err  error
		html template.HTML
		s    goatcounter.WidgetSettings

		Limit    int
		Size     string
		SizeStat goatcounter.HitStats
	}
	Locations struct {
		id   int
		err  error
		html template.HTML
		s    goatcounter.WidgetSettings

		Limit   int
		Country string
		LocStat goatcounter.HitStats
	}
)

func (w *Max) SetSettings(s goatcounter.WidgetSettings) {
	w.s = s
}
func (w *TotalCount) SetSettings(s goatcounter.WidgetSettings) {
	w.s = s
}
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
}
func (w *TotalPages) SetSettings(s goatcounter.WidgetSettings) {
	if x := s["align"].Value; x != nil {
		w.Align = x.(bool)
	}
	if x := s["no-events"].Value; x != nil {
		w.NoEvents = x.(bool)
	}
	w.s = s
}
func (w *TopRefs) SetSettings(s goatcounter.WidgetSettings) {
	if x := s["limit"].Value; x != nil {
		w.Limit = int(x.(float64))
	}
	if x := s["key"].Value; x != nil {
		w.Ref = x.(string)
	}
	w.s = s
}
func (w *Browsers) SetSettings(s goatcounter.WidgetSettings) {
	if x := s["limit"].Value; x != nil {
		w.Limit = int(x.(float64))
	}
	if x := s["key"].Value; x != nil {
		w.Browser = x.(string)
	}
	w.s = s
}
func (w *Systems) SetSettings(s goatcounter.WidgetSettings) {
	if x := s["limit"].Value; x != nil {
		w.Limit = int(x.(float64))
	}
	if x := s["key"].Value; x != nil {
		w.System = x.(string)
	}
	w.s = s
}
func (w *Sizes) SetSettings(s goatcounter.WidgetSettings) {
	if x := s["key"].Value; x != nil {
		w.Size = x.(string)
	}
	w.s = s
}
func (w *Locations) SetSettings(s goatcounter.WidgetSettings) {
	w.s = s
	if x := s["limit"].Value; x != nil {
		w.Limit = int(x.(float64))
	}
	if x := s["key"].Value; x != nil {
		w.Country = x.(string)
	}
}

func (w Max) Name() string        { return "max" }
func (w TotalCount) Name() string { return "totalcount" }
func (w Pages) Name() string      { return "pages" }
func (w TotalPages) Name() string { return "totalpages" }
func (w TopRefs) Name() string    { return "toprefs" }
func (w Browsers) Name() string   { return "browsers" }
func (w Systems) Name() string    { return "systems" }
func (w Sizes) Name() string      { return "sizes" }
func (w Locations) Name() string  { return "locations" }

func (w Max) Type() string        { return "data-only" }
func (w TotalCount) Type() string { return "data-only" }
func (w Pages) Type() string      { return "full-width" }
func (w TotalPages) Type() string { return "full-width" }
func (w TopRefs) Type() string    { return "hchart" }
func (w Browsers) Type() string   { return "hchart" }
func (w Systems) Type() string    { return "hchart" }
func (w Sizes) Type() string      { return "hchart" }
func (w Locations) Type() string  { return "hchart" }

func (w Max) Label() string        { return "" }
func (w TotalCount) Label() string { return "" }
func (w Pages) Label() string      { return "Paths overview" }
func (w TotalPages) Label() string { return "Total site pageviews" }
func (w TopRefs) Label() string    { return "Top referrals" }
func (w Browsers) Label() string   { return "Browser stats" }
func (w Systems) Label() string    { return "System stats" }
func (w Sizes) Label() string      { return "Size stats" }
func (w Locations) Label() string  { return "Location stats" }

func (w *Max) SetHTML(h template.HTML)        {}
func (w *TotalCount) SetHTML(h template.HTML) {}
func (w *Pages) SetHTML(h template.HTML)      { w.html = h }
func (w *TotalPages) SetHTML(h template.HTML) { w.html = h }
func (w *TopRefs) SetHTML(h template.HTML)    { w.html = h }
func (w *Browsers) SetHTML(h template.HTML)   { w.html = h }
func (w *Systems) SetHTML(h template.HTML)    { w.html = h }
func (w *Sizes) SetHTML(h template.HTML)      { w.html = h }
func (w *Locations) SetHTML(h template.HTML)  { w.html = h }

func (w Max) HTML() template.HTML        { return w.html }
func (w TotalCount) HTML() template.HTML { return w.html }
func (w Pages) HTML() template.HTML      { return w.html }
func (w TotalPages) HTML() template.HTML { return w.html }
func (w TopRefs) HTML() template.HTML    { return w.html }
func (w Browsers) HTML() template.HTML   { return w.html }
func (w Systems) HTML() template.HTML    { return w.html }
func (w Sizes) HTML() template.HTML      { return w.html }
func (w Locations) HTML() template.HTML  { return w.html }

func (w *Max) SetErr(h error)        { w.err = h }
func (w *TotalCount) SetErr(h error) { w.err = h }
func (w *Pages) SetErr(h error)      { w.err = h }
func (w *TotalPages) SetErr(h error) { w.err = h }
func (w *TopRefs) SetErr(h error)    { w.err = h }
func (w *Browsers) SetErr(h error)   { w.err = h }
func (w *Systems) SetErr(h error)    { w.err = h }
func (w *Sizes) SetErr(h error)      { w.err = h }
func (w *Locations) SetErr(h error)  { w.err = h }

func (w Max) Err() error        { return w.err }
func (w Refs) Err() error       { return w.err }
func (w TotalCount) Err() error { return w.err }
func (w Pages) Err() error      { return w.err }
func (w TotalPages) Err() error { return w.err }
func (w TopRefs) Err() error    { return w.err }
func (w Browsers) Err() error   { return w.err }
func (w Systems) Err() error    { return w.err }
func (w Sizes) Err() error      { return w.err }
func (w Locations) Err() error  { return w.err }

func (w Max) Settings() goatcounter.WidgetSettings        { return w.s }
func (w TotalCount) Settings() goatcounter.WidgetSettings { return w.s }
func (w Pages) Settings() goatcounter.WidgetSettings      { return w.s }
func (w TotalPages) Settings() goatcounter.WidgetSettings { return w.s }
func (w TopRefs) Settings() goatcounter.WidgetSettings    { return w.s }
func (w Browsers) Settings() goatcounter.WidgetSettings   { return w.s }
func (w Systems) Settings() goatcounter.WidgetSettings    { return w.s }
func (w Sizes) Settings() goatcounter.WidgetSettings      { return w.s }
func (w Locations) Settings() goatcounter.WidgetSettings  { return w.s }

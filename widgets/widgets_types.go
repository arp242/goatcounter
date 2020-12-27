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
		err                error
		html               template.HTML
		Total, TotalUnique int
		TotalUniqueUTC     int
	}
	Max struct {
		err  error
		html template.HTML
		Max  int
	}
)

// Selectable widgets.
type (
	Pages struct {
		LimitPage int `json:"limit_pages"`
		LimitRef  int `json:"limit_ref"`

		err                    error
		html                   template.HTML
		Display, UniqueDisplay int
		More                   bool
		Pages                  goatcounter.HitStats
		Refs                   goatcounter.Stats
		Max                    int
	}
	TotalPages struct {
		err   error
		html  template.HTML
		Max   int
		Total goatcounter.HitStat
	}
	Refs struct {
		err  error
		html template.HTML
		Refs goatcounter.Stats
	}
	TopRefs struct {
		err     error
		html    template.HTML
		TopRefs goatcounter.Stats
	}
	Browsers struct {
		err      error
		html     template.HTML
		Browsers goatcounter.Stats
	}
	Systems struct {
		err     error
		html    template.HTML
		Systems goatcounter.Stats
	}
	Sizes struct {
		err      error
		html     template.HTML
		SizeStat goatcounter.Stats
	}
	Locations struct {
		err     error
		html    template.HTML
		LocStat goatcounter.Stats
	}
)

func (w Max) Name() string        { return "max" }
func (w Refs) Name() string       { return "refs" }
func (w TotalCount) Name() string { return "totalcount" }
func (w Pages) Name() string      { return "pages" }
func (w TotalPages) Name() string { return "totalpages" }
func (w TopRefs) Name() string    { return "toprefs" }
func (w Browsers) Name() string   { return "browsers" }
func (w Systems) Name() string    { return "systems" }
func (w Sizes) Name() string      { return "sizes" }
func (w Locations) Name() string  { return "locations" }

func (w Max) Type() string        { return "data-only" }
func (w Refs) Type() string       { return "data-only" }
func (w TotalCount) Type() string { return "data-only" }
func (w Pages) Type() string      { return "full-width" }
func (w TotalPages) Type() string { return "full-width" }
func (w TopRefs) Type() string    { return "hchart" }
func (w Browsers) Type() string   { return "hchart" }
func (w Systems) Type() string    { return "hchart" }
func (w Sizes) Type() string      { return "hchart" }
func (w Locations) Type() string  { return "hchart" }

func (w Max) Label() string        { return "" }
func (w Refs) Label() string       { return "" }
func (w TotalCount) Label() string { return "" }
func (w Pages) Label() string      { return "Paths overview" }
func (w TotalPages) Label() string { return "Total site pageviews" }
func (w TopRefs) Label() string    { return "Top referrals" }
func (w Browsers) Label() string   { return "Browser stats" }
func (w Systems) Label() string    { return "System stats" }
func (w Sizes) Label() string      { return "Size stats" }
func (w Locations) Label() string  { return "Location stats" }

func (w *Max) SetHTML(h template.HTML)        {}
func (w *Refs) SetHTML(h template.HTML)       {}
func (w *TotalCount) SetHTML(h template.HTML) {}
func (w *Pages) SetHTML(h template.HTML)      { w.html = h }
func (w *TotalPages) SetHTML(h template.HTML) { w.html = h }
func (w *TopRefs) SetHTML(h template.HTML)    { w.html = h }
func (w *Browsers) SetHTML(h template.HTML)   { w.html = h }
func (w *Systems) SetHTML(h template.HTML)    { w.html = h }
func (w *Sizes) SetHTML(h template.HTML)      { w.html = h }
func (w *Locations) SetHTML(h template.HTML)  { w.html = h }

func (w Max) HTML() template.HTML        { return w.html }
func (w Refs) HTML() template.HTML       { return w.html }
func (w TotalCount) HTML() template.HTML { return w.html }
func (w Pages) HTML() template.HTML      { return w.html }
func (w TotalPages) HTML() template.HTML { return w.html }
func (w TopRefs) HTML() template.HTML    { return w.html }
func (w Browsers) HTML() template.HTML   { return w.html }
func (w Systems) HTML() template.HTML    { return w.html }
func (w Sizes) HTML() template.HTML      { return w.html }
func (w Locations) HTML() template.HTML  { return w.html }

func (w *Max) SetErr(h error)        { w.err = h }
func (w *Refs) SetErr(h error)       { w.err = h }
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

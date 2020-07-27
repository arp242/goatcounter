// Copyright © 2019 Martin Tournoij – This file is part of GoatCounter and
// published under the terms of a slightly modified EUPL v1.2 license, which can
// be found in the LICENSE file or at https://license.goatcounter.com

package widgets

import (
	"html/template"

	"zgo.at/goatcounter"
)

type (
	Totals struct {
		html               template.HTML
		Total, TotalUnique int
	}
	AllTotals struct {
		html           template.HTML
		AllTotalUnique int
	}
	Pages struct {
		html                   template.HTML
		Display, UniqueDisplay int
		More                   bool
		Pages                  goatcounter.HitStats
		// TODO: on SharedData for now.
		//Refs                   goatcounter.Stats
		//Max                    int
	}
	Max struct {
		html template.HTML
		Max  int
	}
	Totalpages struct {
		html  template.HTML
		Max   int
		Total goatcounter.HitStat
	}
	Refs struct {
		html template.HTML
		Refs goatcounter.Stats
	}
	Toprefs struct {
		html    template.HTML
		TopRefs goatcounter.Stats
	}
	Browsers struct {
		html     template.HTML
		Browsers goatcounter.Stats
	}
	Systems struct {
		html    template.HTML
		Systems goatcounter.Stats
	}
	Sizes struct {
		html     template.HTML
		SizeStat goatcounter.Stats
	}
	Locations struct {
		html    template.HTML
		LocStat goatcounter.Stats
	}
)

var list = map[string]Widget{
	"totals":     &Totals{},
	"alltotals":  &AllTotals{},
	"pages":      &Pages{},
	"max":        &Max{},
	"totalpages": &Totalpages{},
	"refs":       &Refs{},
	"toprefs":    &Toprefs{},
	"browsers":   &Browsers{},
	"systems":    &Systems{},
	"sizes":      &Sizes{},
	"locations":  &Locations{},
}

func (w AllTotals) Name() string  { return "alltotals" }
func (w Max) Name() string        { return "max" }
func (w Refs) Name() string       { return "refs" }
func (w Totals) Name() string     { return "totals" }
func (w Pages) Name() string      { return "pages" }
func (w Totalpages) Name() string { return "totalpages" }
func (w Toprefs) Name() string    { return "toprefs" }
func (w Browsers) Name() string   { return "browsers" }
func (w Systems) Name() string    { return "systems" }
func (w Sizes) Name() string      { return "sizes" }
func (w Locations) Name() string  { return "locations" }

func (w AllTotals) Type() string  { return "data-only" }
func (w Max) Type() string        { return "data-only" }
func (w Refs) Type() string       { return "data-only" }
func (w Totals) Type() string     { return "data-only" }
func (w Pages) Type() string      { return "full-width" }
func (w Totalpages) Type() string { return "full-width" }
func (w Toprefs) Type() string    { return "hchart" }
func (w Browsers) Type() string   { return "hchart" }
func (w Systems) Type() string    { return "hchart" }
func (w Sizes) Type() string      { return "hchart" }
func (w Locations) Type() string  { return "hchart" }

func (w *AllTotals) SetHTML(h template.HTML)  {}
func (w *Max) SetHTML(h template.HTML)        {}
func (w *Refs) SetHTML(h template.HTML)       {}
func (w *Totals) SetHTML(h template.HTML)     {}
func (w *Pages) SetHTML(h template.HTML)      { w.html = h }
func (w *Totalpages) SetHTML(h template.HTML) { w.html = h }
func (w *Toprefs) SetHTML(h template.HTML)    { w.html = h }
func (w *Browsers) SetHTML(h template.HTML)   { w.html = h }
func (w *Systems) SetHTML(h template.HTML)    { w.html = h }
func (w *Sizes) SetHTML(h template.HTML)      { w.html = h }
func (w *Locations) SetHTML(h template.HTML)  { w.html = h }

func (w AllTotals) HTML() template.HTML  { return w.html }
func (w Max) HTML() template.HTML        { return w.html }
func (w Refs) HTML() template.HTML       { return w.html }
func (w Totals) HTML() template.HTML     { return w.html }
func (w Pages) HTML() template.HTML      { return w.html }
func (w Totalpages) HTML() template.HTML { return w.html }
func (w Toprefs) HTML() template.HTML    { return w.html }
func (w Browsers) HTML() template.HTML   { return w.html }
func (w Systems) HTML() template.HTML    { return w.html }
func (w Sizes) HTML() template.HTML      { return w.html }
func (w Locations) HTML() template.HTML  { return w.html }

func (w AllTotals) Clone() Widget  { return &w }
func (w Max) Clone() Widget        { return &w }
func (w Refs) Clone() Widget       { return &w }
func (w Totals) Clone() Widget     { return &w }
func (w Pages) Clone() Widget      { return &w }
func (w Totalpages) Clone() Widget { return &w }
func (w Toprefs) Clone() Widget    { return &w }
func (w Browsers) Clone() Widget   { return &w }
func (w Systems) Clone() Widget    { return &w }
func (w Sizes) Clone() Widget      { return &w }
func (w Locations) Clone() Widget  { return &w }

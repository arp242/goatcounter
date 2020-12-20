// Copyright © 2019 Martin Tournoij – This file is part of GoatCounter and
// published under the terms of a slightly modified EUPL v1.2 license, which can
// be found in the LICENSE file or at https://license.goatcounter.com

package goatcounter

import (
	"fmt"
	"strconv"
)

func defaultWidgets() Widgets {
	s := defaultWidgetSettings()
	w := Widgets{}
	for _, n := range []string{"pages", "totalpages", "toprefs", "browsers", "systems", "sizes", "locations"} {
		w = append(w, map[string]interface{}{"on": true, "name": n, "s": s[n].getMap()})
	}
	return w
}

func defaultWidgetSettings() map[string]widgetSettings {
	return map[string]widgetSettings{
		"pages": map[string]widgetSetting{
			"limit_pages": widgetSetting{
				Type:  "number",
				Label: "Page size",
				Help:  "Number of pages to load",
				Value: float64(10),
			},
			"limit_refs": widgetSetting{
				Type:  "number",
				Label: "Referrers page size",
				Help:  "Number of referrers to load when clicking on a path",
				Value: float64(10),
			},
		},
		"totalpages": map[string]widgetSetting{
			"align": widgetSetting{
				Type:  "checkbox",
				Label: "Align with pages",
				Help:  "Add margin to the left so it aligns with pages charts",
				Value: false,
			},
			"no-events": widgetSetting{
				Type:  "checkbox",
				Label: "Exclude events",
				Help:  "Don't include events in the Totals overview",
				Value: false,
			},
		},
	}
}

type widgetSetting struct {
	Type  string
	Label string
	Help  string

	Value interface{}
}

type widgetSettings map[string]widgetSetting

func (s widgetSettings) getMap() map[string]interface{} {
	m := make(map[string]interface{})
	for k, v := range s {
		m[k] = v.Value
	}
	return m
}

type Widget map[string]interface{}

func (w Widget) SetSetting(widget, setting, value string) error {
	defW, ok := defaultWidgetSettings()[widget]
	if !ok {
		return fmt.Errorf("Widget.SetSetting: no such widget %q", widget)
	}
	def, ok := defW[setting]
	if !ok {
		return fmt.Errorf("Widget.SetSetting: no such setting %q for widget %q", setting, widget)
	}

	s, ok := w["s"].(map[string]interface{})
	if !ok {
		s = make(map[string]interface{})
	}
	switch def.Type {
	case "number":
		n, err := strconv.Atoi(value)
		if err != nil {
			return err
		}
		s[setting] = float64(n)
	case "checkbox":
		s[setting] = value == "on"
	case "text":
		s[setting] = value
	}
	w["s"] = s
	return nil
}

// Widgets is a list of widgets to be printed, in order.
type Widgets []Widget

// Get a widget from the list by name.
func (w Widgets) Get(name string) Widget {
	for _, v := range w {
		if v["name"] == name {
			return v
		}
	}
	return nil
}

// GetSettings gets all setting for this widget.
func (w Widgets) GetSettings(name string) widgetSettings {
	for _, v := range w {
		if v["name"] == name {
			def := defaultWidgetSettings()[name]
			s, ok := v["s"]
			if ok {
				// {"limit_pages": 10, "limit_refs": 10}},
				ss := s.(map[string]interface{})
				for k, v := range ss {
					d := def[k]
					d.Value = v
					def[k] = d
				}
			}
			return def
		}
	}
	return nil
}

// On reports if this setting should be displayed.
func (w Widgets) On(name string) bool {
	ww := w.Get(name)
	b, ok := ww["on"].(bool)
	if !ok {
		return false
	}
	return b
}

// Some shortcuts for getting the settings.

func (ss SiteSettings) LimitPages() int {
	return int(ss.Widgets.GetSettings("pages")["limit_pages"].Value.(float64))
}
func (ss SiteSettings) LimitRefs() int {
	return int(ss.Widgets.GetSettings("pages")["limit_refs"].Value.(float64))
}
func (ss SiteSettings) TotalsAlign() bool {
	return ss.Widgets.GetSettings("totalpages")["align"].Value.(bool)
}
func (ss SiteSettings) TotalsNoEvents() bool {
	return ss.Widgets.GetSettings("totalpages")["no-events"].Value.(bool)
}

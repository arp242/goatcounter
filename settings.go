// Copyright © 2019 Martin Tournoij – This file is part of GoatCounter and
// published under the terms of a slightly modified EUPL v1.2 license, which can
// be found in the LICENSE file or at https://license.goatcounter.com

package goatcounter

import (
	"database/sql/driver"
	"fmt"
	"strconv"
	"strings"

	"zgo.at/json"
	"zgo.at/tz"
	"zgo.at/zstd/zint"
	"zgo.at/zstd/zjson"
	"zgo.at/zvalidate"
)

// Settings.Collect values (bitmask)
//
// DO NOT change the values of these constants; they're stored in the database.
//
// Note: also update CollectFlags() method below.
//
// Nothing is 1 and 0 is "unset". This is so we can distinguish between "this
// field was never sent in the form" vs. "user unchecked all boxes".
const (
	CollectNothing        zint.Bitflag16 = 1 << iota
	CollectReferrer                      // 2
	CollectUserAgent                     // 4
	CollectScreenSize                    // 8
	CollectLocation                      // 16
	CollectLocationRegion                // 32
	CollectLanguage                      // 64
	CollectSession                       // 128
)

type (
	// SiteSettings contains all the user-configurable settings for a site, with
	// the exception of the domain and billing settings.
	//
	// This is stored as JSON in the database.
	SiteSettings struct {
		Public         bool           `json:"public"`
		AllowCounter   bool           `json:"allow_counter"`
		AllowBosmang   bool           `json:"allow_bosmang"`
		DataRetention  int            `json:"data_retention"`
		Campaigns      Strings        `json:"campaigns"`
		IgnoreIPs      Strings        `json:"ignore_ips"`
		Collect        zint.Bitflag16 `json:"collect"`
		CollectRegions Strings        `json:"collect_regions"`
	}

	// UserSettings are all user preferences.
	UserSettings struct {
		TwentyFourHours  bool     `json:"twenty_four_hours"`
		SundayStartsWeek bool     `json:"sunday_starts_week"`
		DateFormat       string   `json:"date_format"`
		NumberFormat     rune     `json:"number_format"`
		Timezone         *tz.Zone `json:"timezone"`
		Widgets          Widgets  `json:"widgets"`
		Views            Views    `json:"views"`
	}

	// Widgets is a list of widgets to be printed, in order.
	Widgets []Widget
	Widget  map[string]interface{}

	widgetSettings map[string]widgetSetting
	widgetSetting  struct {
		Type  string
		Label string
		Help  string

		Value interface{}
	}

	// Views for the dashboard; these settings apply to all widget and are
	// configurable in the yellow box at the top.
	Views []View
	View  struct {
		Name   string `json:"name"`
		Filter string `json:"filter"`
		Daily  bool   `json:"daily"`
		AsText bool   `json:"as-text"`
		Period string `json:"period"` // "week", "week-cur", or n days: "8"
	}
)

// Default widgets for new sites.
//
// This *must* return a list of all configurable widgets; even if it's off by
// default.
//
// As a function to ensure a global map isn't accidentally modified.
func defaultWidgets() Widgets {
	s := defaultWidgetSettings()
	w := Widgets{}
	for _, n := range []string{"pages", "totalpages", "toprefs", "browsers", "systems", "sizes", "locations"} {
		w = append(w, map[string]interface{}{"on": true, "name": n, "s": s[n].getMap()})
	}
	return w
}

// List of all settings for widgets with some data.
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

func (ss SiteSettings) String() string               { return string(zjson.MustMarshal(ss)) }
func (ss SiteSettings) Value() (driver.Value, error) { return json.Marshal(ss) }
func (ss *SiteSettings) Scan(v interface{}) error {
	switch vv := v.(type) {
	case []byte:
		return json.Unmarshal(vv, ss)
	case string:
		return json.Unmarshal([]byte(vv), ss)
	default:
		return fmt.Errorf("SiteSettings.Scan: unsupported type: %T", v)
	}
}
func (ss UserSettings) String() string               { return string(zjson.MustMarshal(ss)) }
func (ss UserSettings) Value() (driver.Value, error) { return json.Marshal(ss) }
func (ss *UserSettings) Scan(v interface{}) error {
	switch vv := v.(type) {
	case []byte:
		return json.Unmarshal(vv, ss)
	case string:
		return json.Unmarshal([]byte(vv), ss)
	default:
		return fmt.Errorf("UserSettings.Scan: unsupported type: %T", v)
	}
}

func (ss *SiteSettings) Defaults() {
	if ss.Campaigns == nil {
		ss.Campaigns = []string{"utm_campaign", "utm_source", "ref"}
	}

	if ss.Collect == 0 {
		ss.Collect = CollectReferrer | CollectUserAgent | CollectScreenSize | CollectLocation | CollectLocationRegion | CollectSession
	}
	if ss.Collect.Has(CollectLocationRegion) { // Collecting region without country makes no sense.
		ss.Collect |= CollectLocation
	}
	if ss.CollectRegions == nil {
		ss.CollectRegions = []string{"US", "RU", "CH"}
	}
}

func (ss *SiteSettings) Validate() error {
	v := zvalidate.New()

	if ss.DataRetention > 0 {
		v.Range("data_retention", int64(ss.DataRetention), 14, 0)
	}

	if len(ss.IgnoreIPs) > 0 {
		for _, ip := range ss.IgnoreIPs {
			v.IP("ignore_ips", ip)
		}
	}

	return v.ErrorOrNil()
}

type CollectFlag struct {
	Label, Help string
	Flag        zint.Bitflag16
}

// CollectFlags returns a list of all flags we know for the Collect settings.
func (ss SiteSettings) CollectFlags() []CollectFlag {
	return []CollectFlag{
		{
			Label: "Sessions",
			Help:  "Track unique visitors for up to 8 hours; if you disable this then someone pressing e.g. F5 to reload the page will just show as 2 pageviews instead of 1",
			Flag:  CollectSession,
		},
		{
			Label: "Referrer",
			Help:  "Referer header and campaign parameters.",
			Flag:  CollectReferrer,
		},
		{
			Label: "User-Agent",
			Help:  "User-Agent header to get the browser and system name and version.",
			Flag:  CollectUserAgent,
		},
		{
			Label: "Size",
			Help:  "Screen size.",
			Flag:  CollectScreenSize,
		},
		{
			Label: "Country",
			Help:  "Country name, for example Belgium, Indonesia, etc.",
			Flag:  CollectLocation,
		},
		{
			Label: "Region",
			Help:  "Region, for example Texas, Bali, etc. The details for this differ per country.",
			Flag:  CollectLocationRegion,
		},
		// {
		// 	Label: "Language",
		// 	Help:  "Supported languages from Accept-Language",
		// 	Flag:  CollectLanguage,
		// },
	}
}

func (s widgetSettings) getMap() map[string]interface{} {
	m := make(map[string]interface{})
	for k, v := range s {
		m[k] = v.Value
	}
	return m
}

// SetSettings set the setting "setting" for widget "widget" to "value".
//
// The value is converted to the correct type for this setting.
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
			return fmt.Errorf("Widget.SetSetting: setting %q for widget %q: %w", setting, widget, err)
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
					if v != nil {
						d := def[k]
						d.Value = v
						def[k] = d
					}
				}
			}
			return def
		}
	}
	return make(widgetSettings)
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

// Get a view for this site by name and returns the view and index.
// Returns -1 if this view doesn't exist.
func (v Views) Get(name string) (View, int) {
	for i, vv := range v {
		if strings.EqualFold(vv.Name, name) {
			return vv, i
		}
	}
	return View{}, -1
}

func (ss *UserSettings) Defaults() {
	if ss.DateFormat == "" {
		ss.DateFormat = "2 Jan ’06"
	}
	if ss.NumberFormat == 0 {
		ss.NumberFormat = 0x202f
	}
	if ss.Timezone == nil {
		ss.Timezone = tz.UTC
	}

	if len(ss.Widgets) == 0 {
		ss.Widgets = defaultWidgets()
	}
	if len(ss.Views) == 0 {
		ss.Views = Views{{Name: "default", Period: "week"}}
	}
}

func (ss *UserSettings) Validate() error {
	v := zvalidate.New()

	// Must always include all widgets we know about.
	for _, w := range defaultWidgets() {
		if ss.Widgets.Get(w["name"].(string)) == nil {
			v.Append("widgets", fmt.Sprintf("widget %q is missing", w["name"].(string)))
		}
	}
	v.Range("widgets.pages.s.limit_pages", int64(ss.LimitPages()), 1, 100)
	v.Range("widgets.pages.s.limit_refs", int64(ss.LimitRefs()), 1, 25)

	if _, i := ss.Views.Get("default"); i == -1 || len(ss.Views) != 1 {
		v.Append("views", "view not set")
	}

	return v.ErrorOrNil()
}

// Some shortcuts for getting the settings.

func (ss UserSettings) LimitPages() int {
	return int(ss.Widgets.GetSettings("pages")["limit_pages"].Value.(float64))
}
func (ss UserSettings) LimitRefs() int {
	return int(ss.Widgets.GetSettings("pages")["limit_refs"].Value.(float64))
}
func (ss UserSettings) SplitEvents() bool {
	return ss.Widgets.GetSettings("pages")["split_events"].Value.(bool)
}
func (ss UserSettings) TotalsAlign() bool {
	return ss.Widgets.GetSettings("totalpages")["align"].Value.(bool)
}
func (ss UserSettings) TotalsNoEvents() bool {
	return ss.Widgets.GetSettings("totalpages")["no-events"].Value.(bool)
}

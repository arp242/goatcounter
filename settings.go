// Copyright © 2019 Martin Tournoij – This file is part of GoatCounter and
// published under the terms of a slightly modified EUPL v1.2 license, which can
// be found in the LICENSE file or at https://license.goatcounter.com

package goatcounter

import (
	"context"
	"database/sql/driver"
	"fmt"
	"sort"
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
		Language         string   `json:"language"`
		DateFormat       string   `json:"date_format"`
		NumberFormat     rune     `json:"number_format"`
		Timezone         *tz.Zone `json:"timezone"`
		Widgets          Widgets  `json:"widgets"`
		Views            Views    `json:"views"`
	}

	// Widgets is a list of widgets to be printed, in order.
	Widgets []Widget
	Widget  map[string]interface{}

	WidgetSettings map[string]WidgetSetting
	WidgetSetting  struct {
		Type        string
		Hidden      bool
		Label       string
		Help        string
		Options     [][2]string
		OptionsFunc func(context.Context) [][2]string
		Validate    func(*zvalidate.Validator, interface{})
		Value       interface{}
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
		w = append(w, map[string]interface{}{"n": n, "s": s[n].getMap()})
	}
	return w
}

// List of all settings for widgets with some data.
func defaultWidgetSettings() map[string]WidgetSettings {
	return map[string]WidgetSettings{
		"pages": map[string]WidgetSetting{
			"limit_pages": WidgetSetting{
				Type:  "number",
				Label: "Page size",
				Help:  "Number of pages to load",
				Value: float64(10),
				Validate: func(v *zvalidate.Validator, val interface{}) {
					v.Range("limit_pages", int64(val.(float64)), 1, 100)
				},
			},
			"limit_refs": WidgetSetting{
				Type:  "number",
				Label: "Referrers page size",
				Help:  "Number of referrers to load when clicking on a path",
				Value: float64(10),
				Validate: func(v *zvalidate.Validator, val interface{}) {
					v.Range("limit_pages", int64(val.(float64)), 1, 100)
				},
			},
		},
		"totalpages": map[string]WidgetSetting{
			"align": WidgetSetting{
				Type:  "checkbox",
				Label: "Align with pages",
				Help:  "Add margin to the left so it aligns with pages charts",
				Value: false,
			},
			"no-events": WidgetSetting{
				Type:  "checkbox",
				Label: "Exclude events",
				Help:  "Don't include events in the Totals overview",
				Value: false,
			},
		},
		"toprefs": map[string]WidgetSetting{
			"limit": WidgetSetting{
				Type:  "number",
				Label: "Page size",
				Help:  "Number of pages to load",
				Value: float64(6),
				Validate: func(v *zvalidate.Validator, val interface{}) {
					v.Range("limit", int64(val.(float64)), 1, 20)
				},
			},
			"key": WidgetSetting{Hidden: true},
		},
		"browsers": map[string]WidgetSetting{
			"limit": WidgetSetting{
				Type:  "number",
				Label: "Page size",
				Help:  "Number of pages to load",
				Value: float64(6),
				Validate: func(v *zvalidate.Validator, val interface{}) {
					v.Range("limit", int64(val.(float64)), 1, 20)
				},
			},
			"key": WidgetSetting{Hidden: true},
		},
		"systems": map[string]WidgetSetting{
			"limit": WidgetSetting{
				Type:  "number",
				Label: "Page size",
				Help:  "Number of pages to load",
				Value: float64(6),
				Validate: func(v *zvalidate.Validator, val interface{}) {
					v.Range("limit", int64(val.(float64)), 1, 20)
				},
			},
			"key": WidgetSetting{Hidden: true},
		},
		"sizes": map[string]WidgetSetting{
			"key": WidgetSetting{Hidden: true},
		},
		"locations": map[string]WidgetSetting{
			"limit": WidgetSetting{
				Type:  "number",
				Label: "Page size",
				Help:  "Number of pages to load",
				Value: float64(6),
				Validate: func(v *zvalidate.Validator, val interface{}) {
					v.Range("limit", int64(val.(float64)), 1, 20)
				},
			},
			"key": WidgetSetting{
				Type:  "select",
				Label: "Show regions",
				Help:  "Show regions for this country instead of a country list",
				Value: "",
				OptionsFunc: func(ctx context.Context) [][2]string {
					var l Locations
					err := l.ListCountries(ctx)
					if err != nil {
						panic(err)
					}
					countries := make([][2]string, 0, len(l)+1)
					countries = append(countries, [2]string{"", ""})
					for _, ll := range l {
						countries = append(countries, [2]string{ll.Country, ll.CountryName})
					}
					return countries
				},
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
		v.Range("data_retention", int64(ss.DataRetention), 31, 0)
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

func (s *WidgetSettings) Set(k string, v interface{}) {
	ss := *s
	m := ss[k]
	m.Value = v
	ss[k] = m
}

func (s WidgetSettings) getMap() map[string]interface{} {
	m := make(map[string]interface{})
	for k, v := range s {
		m[k] = v.Value
	}
	return m
}

// HasSettings reports if there are any non-hidden settings.
func (s WidgetSettings) HasSettings() bool {
	for _, ss := range s {
		if !ss.Hidden {
			return true
		}
	}
	return false
}

// Display all values that are different from the default.
func (s WidgetSettings) Display(wname string) string {
	defaults := defaultWidgetSettings()[wname]

	order := make([]string, 0, len(s))
	for k := range s {
		order = append(order, k)
	}
	sort.Strings(order)

	str := make([]string, 0, len(s))
	for _, k := range order {
		ss := s[k]
		if ss.Hidden {
			continue
		}
		if ss.Value == defaults[k].Value {
			continue
		}

		l := strings.ToLower(ss.Label)
		if ss.Type == "checkbox" {
			str = append(str, l)
		} else {
			str = append(str, fmt.Sprintf("%s: %v", l, ss.Value))
		}
	}
	return strings.Join(str, ", ")
}

func NewWidget(name string) Widget {
	return Widget{"n": name}
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
	case "text", "select":
		s[setting] = value
	}
	w["s"] = s
	return nil
}

// Name gets this widget's name.
func (w Widget) Name() string { return w["n"].(string) }

func (w Widget) GetSetting(n string) interface{} {
	for k, v := range w.GetSettings() {
		if k == n {
			return v.Value
		}
	}
	return nil
}

// GetSettings gets all setting for this widget.
func (w Widget) GetSettings() WidgetSettings {
	def := defaultWidgetSettings()[w.Name()]
	s, ok := w["s"]
	if ok {
		for k, v := range s.(map[string]interface{}) {
			if v != nil {
				d := def[k]
				d.Value = v
				def[k] = d
			}
		}
	}
	return def
}

// Get all widget from the list by name.
func (w Widgets) Get(name string) Widgets {
	var r Widgets
	for _, v := range w {
		if v["n"] == name {
			r = append(r, v)
		}
	}
	return r
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
	if ss.Language == "" {
		ss.Language = "en-GB"
	}
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

	for i, w := range ss.Widgets {
		for _, s := range w.GetSettings() {
			if s.Validate == nil {
				continue
			}
			vv := zvalidate.New()
			s.Validate(&vv, s.Value)
			v.Sub("widgets", strconv.Itoa(i), vv)
		}
	}

	if _, i := ss.Views.Get("default"); i == -1 || len(ss.Views) != 1 {
		v.Append("views", "view not set")
	}

	return v.ErrorOrNil()
}

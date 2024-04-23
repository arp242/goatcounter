// Copyright © Martin Tournoij – This file is part of GoatCounter and published
// under the terms of a slightly modified EUPL v1.2 license, which can be found
// in the LICENSE file or at https://license.goatcounter.com

package goatcounter

import (
	"context"
	"database/sql/driver"
	"fmt"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"

	"zgo.at/json"
	"zgo.at/tz"
	"zgo.at/z18n"
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
	CollectHits                          // 256
)

// UserSettings.EmailReport values.
const (
	EmailReportNever = iota // Email once after 2 weeks; for new sites.
	EmailReportDaily
	EmailReportWeekly
	EmailReportBiWeekly
	EmailReportMonthly
)

var EmailReports = []int{EmailReportNever, EmailReportDaily, EmailReportWeekly,
	EmailReportBiWeekly, EmailReportMonthly}

type (
	// SiteSettings contains all the user-configurable settings for a site, with
	// the exception of the domain settings.
	//
	// This is stored as JSON in the database.
	SiteSettings struct {
		Public         string         `json:"public"`
		Secret         string         `json:"secret"`
		AllowCounter   bool           `json:"allow_counter"`
		AllowBosmang   bool           `json:"allow_bosmang"`
		DataRetention  int            `json:"data_retention"`
		Campaigns      Strings        `json:"-"`
		IgnoreIPs      Strings        `json:"ignore_ips"`
		Collect        zint.Bitflag16 `json:"collect"`
		CollectRegions Strings        `json:"collect_regions"`
		AllowEmbed     Strings        `json:"allow_embed"`
	}

	// UserSettings are all user preferences.
	UserSettings struct {
		TwentyFourHours       bool      `json:"twenty_four_hours"`
		SundayStartsWeek      bool      `json:"sunday_starts_week"`
		Language              string    `json:"language"`
		DateFormat            string    `json:"date_format"`
		NumberFormat          rune      `json:"number_format"`
		Timezone              *tz.Zone  `json:"timezone"`
		Widgets               Widgets   `json:"widgets"`
		Views                 Views     `json:"views"`
		EmailReports          zint.Int  `json:"email_reports"`
		FewerNumbers          bool      `json:"fewer_numbers"`
		FewerNumbersLockUntil time.Time `json:"fewer_numbers_lock_until"`
		Theme                 string    `json:"theme"`
	}

	// Widgets is a list of widgets to be printed, in order.
	Widgets []Widget
	Widget  map[string]any

	WidgetSettings map[string]WidgetSetting
	WidgetSetting  struct {
		Type        string
		Hidden      bool
		Label       string
		Help        string
		Options     [][2]string
		OptionsFunc func(context.Context) [][2]string
		Validate    func(*zvalidate.Validator, any)
		Value       any
	}

	// Views for the dashboard; these settings apply to all widget and are
	// configurable in the yellow box at the top.
	Views []View
	View  struct {
		Name   string `json:"name"`
		Filter string `json:"filter"`
		Daily  bool   `json:"daily"`
		Period string `json:"period"` // "week", "week-cur", or n days: "8"
	}
)

// Default widgets for new sites.
//
// This *must* return a list of all configurable widgets; even if it's off by
// default.
//
// As a function to ensure a global map isn't accidentally modified.
func defaultWidgets(ctx context.Context) Widgets {
	s := defaultWidgetSettings(ctx)
	w := Widgets{}
	for _, n := range []string{"pages", "totalpages", "toprefs", "campaigns", "browsers", "systems", "locations", "languages", "sizes"} {
		w = append(w, map[string]any{"n": n, "s": s[n].getMap()})
	}
	return w
}

// List of all settings for widgets with some data.
func defaultWidgetSettings(ctx context.Context) map[string]WidgetSettings {
	return map[string]WidgetSettings{
		"pages": map[string]WidgetSetting{
			"limit_pages": WidgetSetting{
				Type:  "number",
				Label: z18n.T(ctx, "widget-setting/label/page-size|Page size"),
				Help:  z18n.T(ctx, "widget-setting/help/page-size|Number of pages to load"),
				Value: float64(10),
				Validate: func(v *zvalidate.Validator, val any) {
					v.Range("limit_pages", int64(val.(float64)), 1, 100)
				},
			},
			"limit_refs": WidgetSetting{
				Type:  "number",
				Label: z18n.T(ctx, "widget-setting/label/ref-page-size|Referrers page size"),
				Help:  z18n.T(ctx, "widget-setting/help/ref-page-size|Number of referrers to load when clicking on a path"),
				Value: float64(10),
				Validate: func(v *zvalidate.Validator, val any) {
					v.Range("limit_pages", int64(val.(float64)), 1, 100)
				},
			},
			// "compare": WidgetSetting{
			// 	Type:  "select",
			// 	Value: "none",
			// 	Label: z18n.T(ctx, "widget-setting/label/compare|Compare"),
			// 	Help:  z18n.T(ctx, "widget-setting/help/compare|Show comparison"),
			// 	Options: [][2]string{
			// 		[2]string{"none", z18n.T(ctx, "widget-settings/none|None")},
			// 		[2]string{"period", z18n.T(ctx, "widget-settings/previous-period|Previous period")},
			// 		[2]string{"quarter", z18n.T(ctx, "widget-settings/previous-quarter|Previous quarter")},
			// 		[2]string{"year", z18n.T(ctx, "widget-settings/pervious-year|Previous year")},
			// 	},
			// },
			"style": WidgetSetting{
				Type:  "select",
				Label: z18n.T(ctx, "widget-setting/label/chart-style|Chart style"),
				Help:  z18n.T(ctx, "widget-setting/help/chart-style|How to draw the charts"),
				Value: "line",
				Options: [][2]string{
					[2]string{"line", z18n.T(ctx, "widget-settings/line-chart|Line chart")},
					[2]string{"bar", z18n.T(ctx, "widget-settings/bar-chart|Bar chart")},
					[2]string{"text", z18n.T(ctx, "widget-settings/text-chart|Text table")},
				},
				Validate: func(v *zvalidate.Validator, val any) {
					v.Include("style", val.(string), []string{"line", "bar", "text"})
				},
			},
		},
		"totalpages": map[string]WidgetSetting{
			"align": WidgetSetting{
				Type:  "checkbox",
				Label: z18n.T(ctx, "widget-setting/label/align|Align with pages"),
				Help:  z18n.T(ctx, "widget-setting/help/align|Add margin to the left so it aligns with pages charts"),
				Value: false,
			},
			"no-events": WidgetSetting{
				Type:  "checkbox",
				Label: z18n.T(ctx, "widget-setting/label/no-events|Exclude events"),
				Help:  z18n.T(ctx, "widget-setting/help/no-events|Don't include events in the Totals overview"),
				Value: false,
			},
			"style": WidgetSetting{
				Type:  "select",
				Label: z18n.T(ctx, "widget-setting/label/chart-style|Chart style"),
				Help:  z18n.T(ctx, "widget-setting/help/chart-style|How to draw the charts"),
				Value: "line",
				Options: [][2]string{
					[2]string{"line", z18n.T(ctx, "widget-settings/line-chart|Line chart")},
					[2]string{"bar", z18n.T(ctx, "widget-settings/bar-chart|Bar chart")},
				},
				Validate: func(v *zvalidate.Validator, val any) {
					v.Include("style", val.(string), []string{"line", "bar"})
				},
			},
		},
		"toprefs": map[string]WidgetSetting{
			"limit": WidgetSetting{
				Type:  "number",
				Label: z18n.T(ctx, "widget-setting/label/page-size|Page size"),
				Help:  z18n.T(ctx, "widget-setting/help/page-size|Number of pages to load"),
				Value: float64(6),
				Validate: func(v *zvalidate.Validator, val any) {
					v.Range("limit", int64(val.(float64)), 1, 20)
				},
			},
			"key": WidgetSetting{Hidden: true},
		},
		"browsers": map[string]WidgetSetting{
			"limit": WidgetSetting{
				Type:  "number",
				Label: z18n.T(ctx, "widget-setting/label/page-size|Page size"),
				Help:  z18n.T(ctx, "widget-setting/help/page-size|Number of pages to load"),
				Value: float64(6),
				Validate: func(v *zvalidate.Validator, val any) {
					v.Range("limit", int64(val.(float64)), 1, 20)
				},
			},
			"key": WidgetSetting{Hidden: true},
		},
		"systems": map[string]WidgetSetting{
			"limit": WidgetSetting{
				Type:  "number",
				Label: z18n.T(ctx, "widget-setting/label/page-size|Page size"),
				Help:  z18n.T(ctx, "widget-setting/help/page-size|Number of pages to load"),
				Value: float64(6),
				Validate: func(v *zvalidate.Validator, val any) {
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
				Label: z18n.T(ctx, "widget-setting/label/page-size|Page size"),
				Help:  z18n.T(ctx, "widget-setting/help/page-size|Number of pages to load"),
				Value: float64(6),
				Validate: func(v *zvalidate.Validator, val any) {
					v.Range("limit", int64(val.(float64)), 1, 20)
				},
			},
			"key": WidgetSetting{
				Type:  "select",
				Label: z18n.T(ctx, "widget-setting/label/regions|Show regions"),
				Help:  z18n.T(ctx, "widget-setting/help/regions|Show regions for this country instead of a country list"),
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
		"languages": map[string]WidgetSetting{
			"limit": WidgetSetting{
				Type:  "number",
				Label: z18n.T(ctx, "widget-setting/label/page-size|Page size"),
				Help:  z18n.T(ctx, "widget-setting/help/page-size|Number of pages to load"),
				Value: float64(6),
				Validate: func(v *zvalidate.Validator, val any) {
					v.Range("limit", int64(val.(float64)), 1, 20)
				},
			},
		},
		"campaigns": map[string]WidgetSetting{
			"limit": WidgetSetting{
				Type:  "number",
				Label: z18n.T(ctx, "widget-setting/label/page-size|Page size"),
				Help:  z18n.T(ctx, "widget-setting/help/page-size|Number of pages to load"),
				Value: float64(6),
				Validate: func(v *zvalidate.Validator, val any) {
					v.Range("limit", int64(val.(float64)), 1, 20)
				},
			},
			"key": WidgetSetting{Hidden: true},
		},
	}
}

func (ss SiteSettings) String() string               { return string(zjson.MustMarshal(ss)) }
func (ss SiteSettings) Value() (driver.Value, error) { return json.Marshal(ss) }
func (ss *SiteSettings) Scan(v any) error {
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
func (ss *UserSettings) Scan(v any) error {
	switch vv := v.(type) {
	case []byte:
		return json.Unmarshal(vv, ss)
	case string:
		return json.Unmarshal([]byte(vv), ss)
	default:
		return fmt.Errorf("UserSettings.Scan: unsupported type: %T", v)
	}
}

// This exists as a work-around because a migration set this column wrong >_<
//
// https://github.com/arp242/goatcounter/issues/569#issuecomment-1042013488
func (w *Widgets) UnmarshalJSON(d []byte) error {
	type alias Widgets
	ww := alias(*w)
	err := json.Unmarshal(d, &ww)
	*w = Widgets(ww)

	if err != nil {
		*w = defaultWidgets(context.Background())
	}
	return nil
}

func (ss *SiteSettings) Defaults(ctx context.Context) {
	if ss.Public == "" {
		ss.Public = "private"
	}
	if ss.Collect == 0 {
		ss.Collect = CollectReferrer | CollectUserAgent | CollectScreenSize | CollectLocation | CollectLocationRegion | CollectSession
	}
	if ss.Collect.Has(CollectLocationRegion) { // Collecting region without country makes no sense.
		ss.Collect |= CollectLocation
	}
	if ss.CollectRegions == nil {
		ss.CollectRegions = []string{"US", "RU", "CN"}
	}
}

func (ss *SiteSettings) Validate(ctx context.Context) error {
	v := NewValidate(ctx)

	v.Include("public", ss.Public, []string{"private", "secret", "public"})
	if ss.Public == "secret" {
		v.Len("secret", ss.Secret, 8, 40)
		v.Contains("secret", ss.Secret, []*unicode.RangeTable{zvalidate.AlphaNumeric}, nil)
	}

	if ss.DataRetention > 0 {
		v.Range("data_retention", int64(ss.DataRetention), 31, 0)
	}

	if len(ss.IgnoreIPs) > 0 {
		for _, ip := range ss.IgnoreIPs {
			v.IP("ignore_ips", ip)
		}
	}
	if len(ss.AllowEmbed) > 0 {
		for _, d := range ss.AllowEmbed {
			if d == "*" {
				v.Append("allow_embed", "'*' is not allowed")
			} else {
				v.URL("allow_embed", d)
			}
		}
	}

	return v.ErrorOrNil()
}

func (ss SiteSettings) CanView(token string) bool {
	return ss.Public == "public" || (ss.Public == "secret" && token == ss.Secret)
}

func (ss SiteSettings) IsPublic() bool {
	return ss.Public == "public"
}

type CollectFlag struct {
	Label, Help string
	Flag        zint.Bitflag16
}

// CollectFlags returns a list of all flags we know for the Collect settings.
func (ss SiteSettings) CollectFlags(ctx context.Context) []CollectFlag {
	return []CollectFlag{
		{
			Label: z18n.T(ctx, "data-collect/label/hits|Individual pageviews"),
			Help:  z18n.T(ctx, "data-collect/help/hits|Store individual pageviews for exports. This doesn’t affect anything else. The API can still be used to export aggregate data."),
			Flag:  CollectHits,
		},
		{
			Label: z18n.T(ctx, "data-collect/label/sessions|Sessions"),
			Help:  z18n.T(ctx, "data-collect/help/sessions|%[Track unique visitors] for up to 8 hours; if you disable this then someone pressing e.g. F5 to reload the page will just show as 2 pageviews instead of 1.", z18n.Tag("a", fmt.Sprintf(`href="%s/help/sessions"`, Config(ctx).BasePath))),
			Flag:  CollectSession,
		},
		{
			Label: z18n.T(ctx, "data-collect/label/referrer|Referrer"),
			Help:  z18n.T(ctx, "data-collect/help/referrer|Referer header and campaign parameters."),
			Flag:  CollectReferrer,
		},
		{
			Label: z18n.T(ctx, "data-collect/label/user-agent|User-Agent"),
			Help:  z18n.T(ctx, "data-collect/help/user-agent|Browser and system name derived from the User-Agent header (the header itself is not stored)."),
			Flag:  CollectUserAgent,
		},
		{
			Label: z18n.T(ctx, "data-collect/label/size|Size"),
			Help:  z18n.T(ctx, "data-collect/help/size|Screen size."),
			Flag:  CollectScreenSize,
		},
		{
			Label: z18n.T(ctx, "data-collect/label/country|Country"),
			Help:  z18n.T(ctx, "data-collect/help/country|Country name, for example Belgium, Indonesia, etc."),
			Flag:  CollectLocation,
		},
		{
			Label: z18n.T(ctx, "data-collect/label/region|Region"),
			Help:  z18n.T(ctx, "data-collect/help/region|Region, for example Texas, Bali, etc. The details for this differ per country."),
			Flag:  CollectLocationRegion,
		},
		{
			Label: z18n.T(ctx, "data-collect/label/language|Language"),
			Help:  z18n.T(ctx, "data-collect/help/language|Supported languages from Accept-Language."),
			Flag:  CollectLanguage,
		},
	}
}

func (s *WidgetSettings) Set(k string, v any) {
	ss := *s
	m := ss[k]
	m.Value = v
	ss[k] = m
}

func (s WidgetSettings) getMap() map[string]any {
	m := make(map[string]any)
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
func (s WidgetSettings) Display(ctx context.Context, wname string) string {
	defaults := defaultWidgetSettings(ctx)[wname]

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
func (w Widget) SetSetting(ctx context.Context, widget, setting, value string) error {
	defW, ok := defaultWidgetSettings(ctx)[widget]
	if !ok {
		return fmt.Errorf("Widget.SetSetting: no such widget %q", widget)
	}
	def, ok := defW[setting]
	if !ok {
		return fmt.Errorf("Widget.SetSetting: no such setting %q for widget %q", setting, widget)
	}

	s, ok := w["s"].(map[string]any)
	if !ok {
		s = make(map[string]any)
	}
	switch def.Type {
	case "number":
		n, _ := strconv.Atoi(value)
		if n > 0 {
			s[setting] = float64(n)
		}
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

func (w Widget) GetSetting(ctx context.Context, n string) any {
	for k, v := range w.GetSettings(ctx) {
		if k == n {
			return v.Value
		}
	}
	return nil
}

// GetSettings gets all setting for this widget.
func (w Widget) GetSettings(ctx context.Context) WidgetSettings {
	def := defaultWidgetSettings(ctx)[w.Name()]
	if def == nil {
		def = make(WidgetSettings)
	}
	s, ok := w["s"]
	if ok {
		for k, v := range s.(map[string]any) {
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

// ByID gets this widget by the position/ID.
func (w Widgets) ByID(id int) Widget {
	return w[id]
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

func (ss *UserSettings) Defaults(ctx context.Context) {
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
		ss.Widgets = defaultWidgets(ctx)
	}
	if len(ss.Views) == 0 {
		ss.Views = Views{{Name: "default", Period: "week"}}
	}
}

func (ss *UserSettings) Validate(ctx context.Context) error {
	v := NewValidate(ctx)

	for i, w := range ss.Widgets {
		for _, s := range w.GetSettings(ctx) {
			if s.Validate == nil {
				continue
			}
			vv := NewValidate(ctx)
			s.Validate(&vv, s.Value)
			v.Sub("widgets", strconv.Itoa(i), vv)
		}
	}

	if _, i := ss.Views.Get("default"); i == -1 || len(ss.Views) != 1 {
		v.Append("views", z18n.T(ctx, "view not set"))
	}

	if !slices.Contains(EmailReports, ss.EmailReports.Int()) {
		v.Append("email_reports", "invalid value")
	}

	v.Include("theme", ss.Theme, []string{"", "light", "dark"})

	return v.ErrorOrNil()
}

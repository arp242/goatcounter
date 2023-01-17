// Copyright © Martin Tournoij – This file is part of GoatCounter and published
// under the terms of a slightly modified EUPL v1.2 license, which can be found
// in the LICENSE file or at https://license.goatcounter.com

package handlers

import (
	"context"
	"fmt"
	"net/http"
	"sort"

	"github.com/go-chi/chi/v5"
	"zgo.at/errors"
	"zgo.at/goatcounter/v2"
	"zgo.at/goatcounter/v2/widgets"
	"zgo.at/guru"
	"zgo.at/tz"
	"zgo.at/zdb"
	"zgo.at/zhttp"
	"zgo.at/zstd/ztime"
	"zgo.at/zvalidate"
)

func (h settings) userPref(verr *zvalidate.Validator) zhttp.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		return zhttp.Template(w, "user_pref.gohtml", struct {
			Globals
			Validate           *zvalidate.Validator
			Timezones          []*tz.Zone
			FewerNumbersLocked bool
		}{newGlobals(w, r), verr, tz.Zones,
			goatcounter.MustGetUser(r.Context()).Settings.FewerNumbersLockUntil.After(ztime.Now())})
	}
}

func (h settings) userPrefSave(w http.ResponseWriter, r *http.Request) error {
	args := struct {
		User             goatcounter.User `json:"user"`
		SetSite          bool             `json:"set_site"`
		FewerNumbersLock string           `json:"fewer_numbers_lock"`
		Theme            string           `json:"theme"`
	}{*User(r.Context()), false, "", ""}
	var (
		oldEmail     = args.User.Email
		oldReports   = args.User.Settings.EmailReports
		oldFewerNums = args.User.Settings.FewerNumbers
	)
	_, err := zhttp.Decode(r, &args)
	if err != nil {
		return err
	}

	args.User.Settings.Theme = args.Theme

	if oldFewerNums && !args.User.Settings.FewerNumbers && args.User.Settings.FewerNumbersLockUntil.After(ztime.Now()) {
		zhttp.FlashError(w, "Nice try")
		return zhttp.SeeOther(w, "/user/pref")
	}

	if args.FewerNumbersLock != "" {
		args.User.Settings.FewerNumbersLockUntil = ztime.Time{ztime.Now()}.
			In(args.User.Settings.Timezone.Location).
			AddPeriod(1, map[string]ztime.Period{"week": ztime.WeekMonday, "month": ztime.Month}[args.FewerNumbersLock]).
			StartOf(ztime.Day).
			Time
	}

	var (
		emailChanged     = goatcounter.Config(r.Context()).GoatcounterCom && oldEmail != args.User.Email
		reportingChanged = goatcounter.Config(r.Context()).GoatcounterCom && oldReports != args.User.Settings.EmailReports
	)
	if reportingChanged {
		args.User.LastReportAt = ztime.Now()
	}

	err = zdb.TX(r.Context(), func(ctx context.Context) error {
		err = args.User.Update(ctx, emailChanged)
		if err != nil {
			return err
		}
		if args.User.AccessSettings() && args.SetSite {
			s := Site(ctx)
			s.UserDefaults = args.User.Settings
			return s.Update(ctx)
		}
		return nil
	})
	if err != nil {
		var vErr *zvalidate.Validator
		if errors.As(err, &vErr) {
			return h.userPref(vErr)(w, r)
		}
		return err
	}

	if emailChanged {
		sendEmailVerify(r.Context(), Site(r.Context()), &args.User, goatcounter.Config(r.Context()).EmailFrom)
	}

	zhttp.Flash(w, T(r.Context(), "notify/saved|Saved!"))
	return zhttp.SeeOther(w, "/user/pref")
}

func (h settings) userDashboardWidget(w http.ResponseWriter, r *http.Request) error {
	return zhttp.Template(w, "_user_dashboard_widgets.gohtml", struct {
		Globals
		Widgets widgets.List

		Validate *zvalidate.Validator
	}{newGlobals(w, r),
		widgets.List{widgets.FromSiteWidget(r.Context(), goatcounter.NewWidget(chi.URLParam(r, "name")))},
		nil})
}

func (h settings) userDashboardID(w http.ResponseWriter, r *http.Request) error {
	v := zvalidate.New()
	id := int(v.Integer("id", chi.URLParam(r, "id")))
	if v.HasErrors() {
		return v
	}

	wid := widgets.FromSiteWidget(r.Context(), User(r.Context()).Settings.Widgets.ByID(id))

	return zhttp.Template(w, "_dashboard_configure_widget.gohtml", struct {
		Globals
		Validate *zvalidate.Validator
		Widget   widgets.Widget
		I        int
	}{newGlobals(w, r), nil, wid, 0})
}

func (h settings) userDashboardIDSave(w http.ResponseWriter, r *http.Request) error {
	v := zvalidate.New()
	id := int(v.Integer("id", chi.URLParam(r, "id")))
	if v.HasErrors() {
		return v
	}

	var args struct {
		Widgets []struct {
			Name  string            `json:"name"`
			Index int               `json:"index"`
			S     map[string]string `json:"s"`
		} `json:"widgets"`
	}
	_, err := zhttp.Decode(r, &args)
	if err != nil {
		return err
	}

	user := User(r.Context())

	if len(args.Widgets) != 1 {
		return fmt.Errorf("invalid number of widgets: %d", len(args.Widgets))
	}
	if id > len(user.Settings.Widgets)-1 {
		return fmt.Errorf("invalid widget ID: %d", id)
	}

	// TODO: name is blank?
	for kk, vv := range args.Widgets[0].S {
		err := user.Settings.Widgets[id].SetSetting(r.Context(), args.Widgets[0].Name, kk, vv)
		if err != nil {
			return err
		}
	}
	err = user.Update(r.Context(), false)
	if err != nil {
		return err
	}

	return zhttp.JSON(w, map[string]string{})

}

func (h settings) userDashboard(verr *zvalidate.Validator) zhttp.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		return zhttp.Template(w, "user_dashboard.gohtml", struct {
			Globals
			Validate      *zvalidate.Validator
			Widgets       widgets.List
			CanAddWidgets widgets.List
		}{newGlobals(w, r), verr,
			widgets.FromSiteWidgets(r.Context(), User(r.Context()).Settings.Widgets, widgets.FilterInternal),
			widgets.ListAllWidgets(),
		})
	}
}

func (h settings) userDashboardSave(w http.ResponseWriter, r *http.Request) error {
	var args struct {
		Reset   bool `json:"reset"`
		SetSite bool `json:"set_site"`
		Widgets []struct {
			Name  string            `json:"name"`
			Index int               `json:"index"`
			S     map[string]string `json:"s"`
		} `json:"widgets"`
	}
	_, err := zhttp.Decode(r, &args)
	if err != nil {
		return err
	}

	user := User(r.Context())

	if args.Reset {
		user.Settings.Widgets = nil
		user.Defaults(r.Context())
		err = user.Update(r.Context(), false)
		if err != nil {
			return err
		}
		zhttp.Flash(w, T(r.Context(), "notify/reset-to-default|Reset to defaults!"))
		return zhttp.SeeOther(w, "/user/dashboard")
	}

	if len(args.Widgets) == 0 {
		return guru.New(400, T(r.Context(), "notify/add-one-thing|Must add at least one thing; an empty dashboard is a bit pointless, no?"))
	}

	sort.Slice(args.Widgets, func(i, j int) bool {
		return args.Widgets[i].Index < args.Widgets[j].Index
	})

	user.Settings.Widgets = make(goatcounter.Widgets, 0, len(args.Widgets))
	for _, v := range args.Widgets {
		if v.Name == "" { // Can include blank entries since the array indexes aren't reordered.
			continue
		}
		w := goatcounter.Widget{"n": v.Name}
		for kk, vv := range v.S {
			w.SetSetting(r.Context(), v.Name, kk, vv)
		}
		user.Settings.Widgets = append(user.Settings.Widgets, w)
	}

	err = zdb.TX(r.Context(), func(ctx context.Context) error {
		err = user.Update(ctx, false)
		if err != nil {
			return err
		}
		if user.AccessSettings() && args.SetSite {
			s := Site(ctx)
			s.UserDefaults = user.Settings
			return s.Update(ctx)
		}
		return nil
	})
	if err != nil {
		var v *zvalidate.Validator
		if errors.As(err, &v) {
			return h.userDashboard(v)(w, r)
		}
		return err
	}

	zhttp.Flash(w, "Saved!")
	return zhttp.SeeOther(w, "/user/dashboard")
}

func (h settings) userAuth(verr *zvalidate.Validator) zhttp.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		return zhttp.Template(w, "user_auth.gohtml", struct {
			Globals
			Validate *zvalidate.Validator
		}{newGlobals(w, r), verr})
	}
}

func (h settings) userAPI(verr *zvalidate.Validator) zhttp.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		var tokens goatcounter.APITokens
		err := tokens.List(r.Context())
		if err != nil {
			return err
		}

		return zhttp.Template(w, "user_api.gohtml", struct {
			Globals
			Validate  *zvalidate.Validator
			APITokens goatcounter.APITokens
			Empty     goatcounter.APIToken
		}{newGlobals(w, r), verr, tokens, goatcounter.APIToken{}})
	}
}

func (h settings) userViewSave(w http.ResponseWriter, r *http.Request) error {
	user := User(r.Context())

	v, i := user.Settings.Views.Get("default") // TODO: only default view for now.
	_, err := zhttp.Decode(r, &v)
	if err != nil {
		return err
	}

	user.Settings.Views[i] = v
	err = user.Update(r.Context(), false)
	if err != nil {
		return err
	}

	return zhttp.JSON(w, map[string]string{})
}

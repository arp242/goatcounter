// Copyright © Martin Tournoij – This file is part of GoatCounter and published
// under the terms of a slightly modified EUPL v1.2 license, which can be found
// in the LICENSE file or at https://license.goatcounter.com

package handlers

import (
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/monoculum/formam/v3"
	"zgo.at/bgrun"
	"zgo.at/blackmail"
	"zgo.at/errors"
	"zgo.at/goatcounter/v2"
	"zgo.at/goatcounter/v2/acme"
	"zgo.at/goatcounter/v2/cron"
	"zgo.at/guru"
	"zgo.at/zdb"
	"zgo.at/zhttp"
	"zgo.at/zhttp/header"
	"zgo.at/zhttp/mware"
	"zgo.at/zlog"
	"zgo.at/zstd/zint"
	"zgo.at/zstd/zruntime"
	"zgo.at/zstd/ztime"
	"zgo.at/zvalidate"
)

type settings struct{}

func (h settings) mount(r chi.Router) {
	{ // User settings.
		r.Get("/user", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			zhttp.SeeOther(w, "/user/pref")
		}))

		r.Get("/user/pref", zhttp.Wrap(h.userPref(nil)))
		r.Post("/user/pref", zhttp.Wrap(h.userPrefSave))

		r.Get("/user/dashboard", zhttp.Wrap(h.userDashboard(nil)))
		r.Get("/user/dashboard/widget/{name}", zhttp.Wrap(h.userDashboardWidget))
		r.Get("/user/dashboard/{id}", zhttp.Wrap(h.userDashboardID))
		r.Post("/user/dashboard/{id}", zhttp.Wrap(h.userDashboardIDSave))
		r.Post("/user/dashboard", zhttp.Wrap(h.userDashboardSave))
		r.Post("/user/view", zhttp.Wrap(h.userViewSave))

		r.Get("/user/auth", zhttp.Wrap(h.userAuth(nil)))
	}

	{ // Site settings.
		r.With(requireAccess(goatcounter.AccessSuperuser)).Get("/settings/server", zhttp.Wrap(h.bosmang))
		set := r.With(requireAccess(goatcounter.AccessSettings))

		set.Get("/settings", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			zhttp.SeeOther(w, "/settings/main")
		}))
		set.Get("/settings/main", zhttp.Wrap(func(w http.ResponseWriter, r *http.Request) error {
			return h.main(nil)(w, r)
		}))
		set.Post("/settings/main", zhttp.Wrap(h.mainSave))
		set.Get("/settings/main/ip", zhttp.Wrap(h.ip))
		set.Get("/settings/change-code", zhttp.Wrap(h.changeCode))
		set.Post("/settings/change-code", zhttp.Wrap(h.changeCode))

		set.Get("/settings/purge", zhttp.Wrap(h.purge))
		set.Post("/settings/purge", zhttp.Wrap(h.purgeDo))
		set.Post("/settings/merge", zhttp.Wrap(h.merge))

		set.Get("/settings/export", zhttp.Wrap(func(w http.ResponseWriter, r *http.Request) error {
			return h.export(nil)(w, r)
		}))
		set.Get("/settings/export/{id}", zhttp.Wrap(h.exportDownload))
		set.Post("/settings/export/import", zhttp.Wrap(h.exportImport))
		set.With(mware.Ratelimit(mware.RatelimitOptions{
			Client: mware.RatelimitIP,
			Store:  mware.NewRatelimitMemory(),
			Limit:  rateLimits.export,
			// TODO(i18n): this should be translated, but no locale here; should
			// change the ratelimiter to accept a callback.
			Message: "you can request only one export per hour",
		})).Post("/settings/export", zhttp.Wrap(h.exportStart))
	}

	{ // Admin settings
		admin := r.With(requireAccess(goatcounter.AccessAdmin))

		admin.Get("/user/api", zhttp.Wrap(func(w http.ResponseWriter, r *http.Request) error {
			return h.userAPI(nil)(w, r)
		}))

		admin.Get("/settings/sites", zhttp.Wrap(func(w http.ResponseWriter, r *http.Request) error {
			return h.sites(nil)(w, r)
		}))
		admin.Post("/settings/sites/add", zhttp.Wrap(h.sitesAdd))
		admin.Get("/settings/sites/remove/{id}", zhttp.Wrap(h.sitesRemoveConfirm))
		admin.Post("/settings/sites/remove/{id}", zhttp.Wrap(h.sitesRemove))
		admin.Post("/settings/sites/copy-settings", zhttp.Wrap(h.sitesCopySettings))

		admin.Get("/settings/users", zhttp.Wrap(func(w http.ResponseWriter, r *http.Request) error {
			return h.users(nil)(w, r)
		}))

		admin.Get("/settings/users/add", zhttp.Wrap(func(w http.ResponseWriter, r *http.Request) error {
			return h.usersForm(nil, nil)(w, r)
		}))
		admin.Get("/settings/users/{id}", zhttp.Wrap(func(w http.ResponseWriter, r *http.Request) error {
			return h.usersForm(nil, nil)(w, r)
		}))

		admin.Post("/settings/users/add", zhttp.Wrap(h.usersAdd))
		admin.Post("/settings/users/{id}", zhttp.Wrap(h.usersEdit))
		admin.Post("/settings/users/remove/{id}", zhttp.Wrap(h.usersRemove))

		admin.Get("/settings/delete-account", zhttp.Wrap(func(w http.ResponseWriter, r *http.Request) error {
			return h.delete(nil)(w, r)
		}))
		admin.Post("/settings/delete-account", zhttp.Wrap(h.deleteDo))
	}

}

func (h settings) main(verr *zvalidate.Validator) zhttp.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		return zhttp.Template(w, "settings_main.gohtml", struct {
			Globals
			Validate *zvalidate.Validator
		}{newGlobals(w, r), verr})
	}
}

func (h settings) ip(w http.ResponseWriter, r *http.Request) error {
	return zhttp.String(w, r.RemoteAddr)
}

func (h settings) mainSave(w http.ResponseWriter, r *http.Request) error {
	v := goatcounter.NewValidate(r.Context())

	args := struct {
		Cname      string                   `json:"cname"`
		LinkDomain string                   `json:"link_domain"`
		Settings   goatcounter.SiteSettings `json:"settings"`
	}{}
	_, err := zhttp.Decode(r, &args)
	if err != nil {
		ferr, ok := err.(*formam.Error)
		if !ok || ferr.Code() != formam.ErrCodeConversion {
			return err
		}
		v.Append(ferr.Path(), "must be a number")

		// TODO: we return here because formam stops decoding on the first
		// error. We should really fix this in formam, but it's an incompatible
		// change.
		return h.main(&v)(w, r)
	}

	site := Site(r.Context())
	site.Settings = args.Settings
	site.LinkDomain = args.LinkDomain

	makecert := false
	if args.Cname == "" {
		site.Cname = nil
	} else {
		if site.Cname == nil || *site.Cname != args.Cname {
			makecert = true // Make after we persisted to DB.
		}
		site.Cname = &args.Cname
	}

	err = site.Update(r.Context())
	if err != nil {
		var vErr *zvalidate.Validator
		if !errors.As(err, &vErr) {
			return err
		}
		v.Sub("site", "", err)
	}

	if v.HasErrors() {
		return h.main(&v)(w, r)
	}

	if makecert {
		ctx := goatcounter.CopyContextValues(r.Context())
		bgrun.RunFunction(fmt.Sprintf("acme.Make:%s", args.Cname), func() {
			err := acme.Make(ctx, args.Cname)
			if err != nil {
				zlog.Field("domain", args.Cname).Error(err)
				return
			}

			err = site.UpdateCnameSetupAt(ctx)
			if err != nil {
				zlog.Field("domain", args.Cname).Error(err)
			}
		})
	}

	zhttp.Flash(w, T(r.Context(), "notify/saved|Saved!"))
	return zhttp.SeeOther(w, "/settings")
}

func (h settings) changeCode(w http.ResponseWriter, r *http.Request) error {
	if r.Method == "GET" {
		return zhttp.Template(w, "settings_changecode.gohtml", struct {
			Globals
		}{newGlobals(w, r)})
	}

	var args struct {
		Code string `json:"code"`
	}
	_, err := zhttp.Decode(r, &args)
	if err != nil {
		return err
	}

	site := Site(r.Context())
	err = site.UpdateCode(r.Context(), args.Code)
	if err != nil {
		return err
	}

	zhttp.Flash(w, T(r.Context(), "notify/saved|Saved!"))
	return zhttp.SeeOther(w, site.URL(r.Context())+"/settings/main")
}

func (h settings) sites(verr *zvalidate.Validator) zhttp.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		var sites goatcounter.Sites
		err := sites.ForThisAccount(r.Context(), false)
		if err != nil {
			return err
		}

		return zhttp.Template(w, "settings_sites.gohtml", struct {
			Globals
			SubSites goatcounter.Sites
			Validate *zvalidate.Validator
		}{newGlobals(w, r), sites, verr})
	}
}

func (h settings) sitesAdd(w http.ResponseWriter, r *http.Request) error {
	var args struct {
		Code  string `json:"code"`
		Cname string `json:"cname"`
	}
	_, err := zhttp.Decode(r, &args)
	if err != nil {
		return err
	}

	account := Account(r.Context())

	var (
		newSite goatcounter.Site
		addr    = args.Code
	)
	if goatcounter.Config(r.Context()).GoatcounterCom {
		newSite.Code = args.Code
	} else {
		newSite.Cname = &args.Cname
		addr = args.Cname
	}

	// Undelete previous soft-deleted site.
	id, err := newSite.Exists(r.Context())
	if err != nil {
		return err
	}
	if id > 0 {
		err := newSite.ByIDState(r.Context(), id, goatcounter.StateDeleted)
		if err != nil {
			if zdb.ErrNoRows(err) {
				return guru.Errorf(400, T(r.Context(), "error/address-exists|%(addr) already exists", addr))
			}
			return err
		}
		if newSite.Parent == nil || *newSite.Parent != account.ID {
			return guru.Errorf(400, T(r.Context(), "error/address-exists|%(addr) already exists", addr))
		}

		err = newSite.Undelete(r.Context(), newSite.ID)
		if err != nil {
			return err
		}

		zhttp.Flash(w, T(r.Context(), "notify/restored-previously-deleted-site|Site ‘%(url)’ was previously deleted; restored site with all data.", newSite.URL(r.Context())))
		return zhttp.SeeOther(w, "/settings/sites")
	}

	// Create new site.
	newSite.Parent = &account.ID
	newSite.Settings = Site(r.Context()).Settings
	err = zdb.TX(r.Context(), func(ctx context.Context) error {
		err = newSite.Insert(ctx)
		if err != nil {
			return err
		}
		if !goatcounter.Config(r.Context()).GoatcounterCom {
			return newSite.UpdateCnameSetupAt(ctx)
		}
		return nil
	})
	if err != nil {
		zhttp.FlashError(w, err.Error())
		return zhttp.SeeOther(w, "/settings/sites")
	}

	zhttp.Flash(w, T(r.Context(), "notify/site-added|Site ‘%(url)’ added.", newSite.URL(r.Context())))
	return zhttp.SeeOther(w, "/settings/sites")
}

func (h settings) getSite(ctx context.Context, id int64) (*goatcounter.Site, error) {
	var s goatcounter.Site
	err := s.ByID(ctx, id)
	if err != nil {
		return nil, err
	}

	var account goatcounter.Sites
	err = account.ForThisAccount(ctx, false)
	if err != nil {
		return nil, err
	}

	if !slices.Contains(account.IDs(), s.ID) {
		return nil, guru.New(404, T(ctx, "error/not-found|Not Found"))
	}

	return &s, nil
}

func (h settings) sitesRemoveConfirm(w http.ResponseWriter, r *http.Request) error {
	v := goatcounter.NewValidate(r.Context())
	id := v.Integer("id", chi.URLParam(r, "id"))
	if v.HasErrors() {
		return v
	}

	s, err := h.getSite(r.Context(), id)
	if err != nil {
		return err
	}

	return zhttp.Template(w, "settings_sites_rm_confirm.gohtml", struct {
		Globals
		Rm *goatcounter.Site
	}{newGlobals(w, r), s})
}

func (h settings) sitesRemove(w http.ResponseWriter, r *http.Request) error {
	v := goatcounter.NewValidate(r.Context())
	id := v.Integer("id", chi.URLParam(r, "id"))
	if v.HasErrors() {
		return v
	}

	s, err := h.getSite(r.Context(), id)
	if err != nil {
		return err
	}

	sID := s.ID
	err = s.Delete(r.Context(), false)
	if err != nil {
		return err
	}

	zhttp.Flash(w, T(r.Context(), "notify/site-removed|Site ‘%(url)’ removed.", s.URL(r.Context())))

	// Redirect to parent if we're removing the current site.
	if sID == Site(r.Context()).ID && s.Parent != nil {
		var parent goatcounter.Site
		err = parent.ByID(r.Context(), *s.Parent)
		if err != nil {
			zlog.Error(err)
			return zhttp.SeeOther(w, "/")
		}
		return zhttp.SeeOther(w, parent.URL(r.Context()))
	}
	return zhttp.SeeOther(w, "/settings/sites")
}

func (h settings) sitesCopySettings(w http.ResponseWriter, r *http.Request) error {
	master := Site(r.Context())

	var args struct {
		Sites    []int64 `json:"sites"`
		AllSites bool    `json:"allsites"`
	}
	_, err := zhttp.Decode(r, &args)
	if err != nil {
		return err
	}

	var copies goatcounter.Sites
	if args.AllSites {
		err := copies.ForThisAccount(r.Context(), true)
		if err != nil {
			return err
		}
	} else {
		for _, c := range args.Sites {
			var s goatcounter.Site
			err := s.ByID(r.Context(), c)
			if err != nil {
				return err
			}
			if s.Parent == nil || *s.Parent != master.ID {
				return guru.Errorf(http.StatusForbidden, "yeah nah, site %d doesn't belong to you", s.ID)
			}
			copies = append(copies, s)
		}
	}

	for _, c := range copies {
		c.Settings = master.Settings
		err := c.Update(r.Context())
		if err != nil {
			return err
		}
	}

	zhttp.Flash(w, T(r.Context(), "notify/settings-copied-to-site|Settings copied to the selected sites."))
	return zhttp.SeeOther(w, "/settings/sites")
}

func (h settings) purge(w http.ResponseWriter, r *http.Request) error {
	var (
		path       = strings.TrimSpace(r.URL.Query().Get("path"))
		matchTitle = r.URL.Query().Get("match-title") == "on"
		matchCase  = r.URL.Query().Get("match-case") == "on"
		list       goatcounter.HitLists
		paths      goatcounter.Paths
	)

	if path != "" {
		err := list.ListPathsLike(r.Context(), path, matchTitle, matchCase)
		if err != nil {
			return err
		}

		_, err = paths.List(r.Context(), goatcounter.MustGetSite(r.Context()).ID, 0, 5_000)
		if err != nil {
			return err
		}
	}

	return zhttp.Template(w, "settings_purge.gohtml", struct {
		Globals
		PurgePath  string
		MatchTitle bool
		MatchCase  bool
		List       goatcounter.HitLists
		AllPaths   goatcounter.Paths
	}{newGlobals(w, r), path, matchTitle, matchCase, list, paths})
}

func (h settings) purgeDo(w http.ResponseWriter, r *http.Request) error {
	paths, err := zint.Split(r.Form.Get("paths"), ",")
	if err != nil {
		return err
	}

	ctx := goatcounter.CopyContextValues(r.Context())
	bgrun.RunFunction(fmt.Sprintf("purge:%d", Site(ctx).ID), func() {
		var list goatcounter.Hits
		err := list.Purge(ctx, paths)
		if err != nil {
			zlog.Error(err)
		}
	})

	zhttp.Flash(w, T(r.Context(), "notify/started-background-process|Started in the background; may take about 10-20 seconds to fully process."))
	return zhttp.SeeOther(w, "/settings/purge")
}

func (h settings) merge(w http.ResponseWriter, r *http.Request) error {
	paths, err := zint.Split(r.Form.Get("paths"), ",")
	if err != nil {
		return err
	}

	v := goatcounter.NewValidate(r.Context())
	dst := v.Integer("merge_with", r.Form.Get("merge_with"))
	if v.HasErrors() {
		return v
	}
	paths = slices.DeleteFunc(paths, func(p int64) bool { return p == dst })

	ctx := goatcounter.CopyContextValues(r.Context())
	bgrun.RunFunction(fmt.Sprintf("merge:%d", Site(ctx).ID), func() {
		var list goatcounter.Hits
		err := list.Merge(ctx, dst, paths)
		if err != nil {
			zlog.Error(err)
		}
	})

	zhttp.Flash(w, T(r.Context(), "notify/started-background-process|Started in the background; may take about 10-20 seconds to fully process."))
	return zhttp.SeeOther(w, "/settings/purge")
}

func (h settings) export(verr *zvalidate.Validator) zhttp.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		var exports goatcounter.Exports
		err := exports.List(r.Context())
		if err != nil {
			return err
		}

		ch := goatcounter.MustGetSite(r.Context()).Settings.Collect.Has(goatcounter.CollectHits)
		return zhttp.Template(w, "settings_export.gohtml", struct {
			Globals
			Validate    *zvalidate.Validator
			CollectHits bool
			Exports     goatcounter.Exports
		}{newGlobals(w, r), verr, ch, exports})
	}
}

func (h settings) exportDownload(w http.ResponseWriter, r *http.Request) error {
	v := goatcounter.NewValidate(r.Context())
	id := v.Integer("id", chi.URLParam(r, "id"))
	if v.HasErrors() {
		return v
	}

	var export goatcounter.Export
	err := export.ByID(r.Context(), id)
	if err != nil {
		return err
	}

	fp, err := os.Open(export.Path)
	if err != nil {
		if os.IsNotExist(err) {
			zhttp.FlashError(w, T(r.Context(), "error/export-expired|It looks like there is no export yet or the export has expired."))
			return zhttp.SeeOther(w, "/settings/export")
		}

		return err
	}
	defer fp.Close()

	err = header.SetContentDisposition(w.Header(), header.DispositionArgs{
		Type:     header.TypeAttachment,
		Filename: filepath.Base(export.Path),
	})
	if err != nil {
		return err
	}

	w.Header().Set("Content-Type", "application/gzip")
	return zhttp.Stream(w, fp)
}

func (h settings) exportImport(w http.ResponseWriter, r *http.Request) error {
	v := goatcounter.NewValidate(r.Context())
	replace := v.Boolean("replace", r.Form.Get("replace"))
	if v.HasErrors() {
		return v
	}

	file, head, err := r.FormFile("csv")
	if err != nil {
		return err
	}
	defer file.Close()

	var fp io.ReadCloser = file
	if strings.HasSuffix(head.Filename, ".gz") {
		fp, err = gzip.NewReader(file)
		if err != nil {
			return guru.Errorf(400, T(r.Context(), "error/could-not-read|Could not read as gzip: %(err)", err))
		}
	}
	defer fp.Close()

	user := User(r.Context())
	ctx := goatcounter.CopyContextValues(r.Context())
	n := 0
	bgrun.RunFunction(fmt.Sprintf("import:%d", Site(ctx).ID), func() {
		firstHitAt, err := goatcounter.Import(ctx, fp, replace, true, func(hit goatcounter.Hit, final bool) {
			if final {
				return
			}

			goatcounter.Memstore.Append(hit)
			n++

			// Spread out the load a bit.
			if n%5000 == 0 {
				err := cron.TaskPersistAndStat()
				if err != nil {
					zlog.Error(err)
				}
				cron.WaitPersistAndStat()
			}
		})
		if err != nil {
			if e, ok := err.(*errors.StackErr); ok {
				err = e.Unwrap()
			}

			sendErr := blackmail.Send("GoatCounter import error",
				blackmail.From("GoatCounter import", goatcounter.Config(r.Context()).EmailFrom),
				blackmail.To(user.Email),
				blackmail.BodyMustText(goatcounter.TplEmailImportError{r.Context(), err}.Render))
			if sendErr != nil {
				zlog.Error(sendErr)
			}
		}

		if firstHitAt != nil && !firstHitAt.IsZero() {
			err := Site(ctx).UpdateFirstHitAt(ctx, *firstHitAt)
			if err != nil {
				zlog.Error(err)
			}
		}
	})

	zhttp.Flash(w, T(r.Context(), "notify/import-started-in-background|Import started in the background; you’ll get an email when it’s done."))
	return zhttp.SeeOther(w, "/settings/export")
}

func (h settings) exportStart(w http.ResponseWriter, r *http.Request) error {
	r.ParseForm()

	v := goatcounter.NewValidate(r.Context())
	startFrom := v.Integer("startFrom", r.Form.Get("startFrom"))
	if v.HasErrors() {
		return v
	}

	var export goatcounter.Export
	fp, err := export.Create(r.Context(), startFrom)
	if err != nil {
		return err
	}

	ctx := goatcounter.CopyContextValues(r.Context())
	bgrun.RunFunction(fmt.Sprintf("export web:%d", Site(ctx).ID),
		func() { export.Run(ctx, fp, true) })

	zhttp.Flash(w, T(r.Context(), "notify/export-started-in-background|Export started in the background; you’ll get an email with a download link when it’s done."))
	return zhttp.SeeOther(w, "/settings/export")
}

func (h settings) delete(verr *zvalidate.Validator) zhttp.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		del := map[string]any{
			"ContactMe": r.URL.Query().Get("contact_me") == "true",
			"Reason":    r.URL.Query().Get("reason"),
		}

		var sites goatcounter.Sites
		err := sites.ForThisAccount(r.Context(), false)
		if err != nil {
			return err
		}

		return zhttp.Template(w, "settings_delete.gohtml", struct {
			Globals
			Sites    goatcounter.Sites
			Validate *zvalidate.Validator
			Delete   map[string]any
		}{newGlobals(w, r), sites, verr, del})
	}
}

func (h settings) deleteDo(w http.ResponseWriter, r *http.Request) error {
	var args struct {
		Reason    string `json:"reason"`
		ContactMe bool   `json:"contact_me"`
	}
	_, err := zhttp.Decode(r, &args)
	if err != nil {
		zlog.Error(err)
	}

	account := Account(r.Context())

	if args.Reason != "" {
		bgrun.RunFunction("email:deletion", func() {
			contact := "false"
			if args.ContactMe {
				u := goatcounter.GetUser(r.Context())
				contact = u.Email
			}

			blackmail.Send("GoatCounter deletion",
				blackmail.From("GoatCounter deletion", goatcounter.Config(r.Context()).EmailFrom),
				blackmail.To(goatcounter.Config(r.Context()).EmailFrom),
				blackmail.Bodyf(`Deleted: %s (%d): contact_me: %s; reason: %s`,
					account.Code, account.ID, contact, args.Reason))
		})
	}

	err = account.Delete(r.Context(), true)
	if err != nil {
		return err
	}
	return zhttp.SeeOther(w, "https://"+goatcounter.Config(r.Context()).Domain)
}

func (h settings) users(verr *zvalidate.Validator) zhttp.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		account := Account(r.Context())

		var users goatcounter.Users
		err := users.List(r.Context(), account.ID)
		if err != nil {
			return err
		}

		return zhttp.Template(w, "settings_users.gohtml", struct {
			Globals
			Users    goatcounter.Users
			Validate *zvalidate.Validator
		}{newGlobals(w, r), users, verr})
	}
}

func (h settings) usersForm(newUser *goatcounter.User, pErr error) zhttp.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		edit := newUser != nil && newUser.ID > 0
		if newUser == nil {
			newUser = &goatcounter.User{
				Access: goatcounter.UserAccesses{"all": goatcounter.AccessSettings},
			}

			v := goatcounter.NewValidate(r.Context())
			id := v.Integer("id", chi.URLParam(r, "id"))
			if v.HasErrors() {
				return v
			}
			if id > 0 {
				edit = true
				err := newUser.ByID(r.Context(), id)
				if err != nil {
					return err
				}
			}
		}

		var vErr *zvalidate.Validator
		if errors.As(pErr, &vErr) {
			pErr = nil
		}
		if pErr != nil {
			zlog.Error(pErr)
			var code int
			code, pErr = zhttp.UserError(pErr)
			w.WriteHeader(code)
		}

		return zhttp.Template(w, "settings_users_form.gohtml", struct {
			Globals
			NewUser  goatcounter.User
			Validate *zvalidate.Validator
			Error    error
			Edit     bool
		}{newGlobals(w, r), *newUser, vErr, pErr, edit})
	}
}

func (h settings) usersAdd(w http.ResponseWriter, r *http.Request) error {
	var args struct {
		Email    string                   `json:"email"`
		Password string                   `json:"password"`
		Access   goatcounter.UserAccesses `json:"access"`
	}
	_, err := zhttp.Decode(r, &args)
	if err != nil {
		return err
	}

	account := Account(r.Context())

	newUser := goatcounter.User{
		Email:  args.Email,
		Site:   account.ID,
		Access: args.Access,
	}
	if args.Password != "" {
		newUser.Password = []byte(args.Password)
	}
	if !goatcounter.Config(r.Context()).GoatcounterCom {
		newUser.EmailVerified = true
	}

	err = zdb.TX(r.Context(), func(ctx context.Context) error {
		err := newUser.Insert(ctx, args.Password == "")
		if err != nil {
			return err
		}
		if args.Password == "" {
			return newUser.InviteToken(ctx)
		}
		return nil
	})
	if err != nil {
		return h.usersForm(&newUser, err)(w, r)
	}

	ctx := goatcounter.CopyContextValues(r.Context())
	bgrun.RunFunction(fmt.Sprintf("adduser:%d", newUser.ID), func() {
		err := blackmail.Send(fmt.Sprintf("A GoatCounter account was created for you at %s", account.Display(ctx)),
			blackmail.From("GoatCounter", goatcounter.Config(r.Context()).EmailFrom),
			blackmail.To(newUser.Email),
			blackmail.BodyMustText(goatcounter.TplEmailAddUser{ctx, *account, newUser, goatcounter.GetUser(ctx).Email}.Render),
		)
		if err != nil {
			zlog.Errorf(": %s", err)
		}
	})

	zhttp.Flash(w, T(r.Context(), "notify/user-added|User ‘%(email)’ added.", newUser.Email))
	return zhttp.SeeOther(w, "/settings/users")
}

func (h settings) usersEdit(w http.ResponseWriter, r *http.Request) error {
	v := goatcounter.NewValidate(r.Context())
	id := v.Integer("id", chi.URLParam(r, "id"))
	if v.HasErrors() {
		return v
	}

	var args struct {
		Email    string                   `json:"email"`
		Password string                   `json:"password"`
		Access   goatcounter.UserAccesses `json:"access"`
	}
	_, err := zhttp.Decode(r, &args)
	if err != nil {
		return err
	}

	if args.Access["all"] == goatcounter.AccessSuperuser && !User(r.Context()).AccessSuperuser() {
		return guru.New(400, "can't set 'superuser' if you're not a superuser yourself.")
	}

	var editUser goatcounter.User
	err = editUser.ByID(r.Context(), id)
	if err != nil {
		return err
	}

	account := Account(r.Context())
	if account.ID != editUser.Site {
		return guru.New(404, T(r.Context(), "notify/not-found|Not Found"))
	}

	emailChanged := editUser.Email != args.Email
	editUser.Email = args.Email
	editUser.Access = args.Access

	err = zdb.TX(r.Context(), func(ctx context.Context) error {
		err = editUser.Update(ctx, emailChanged)
		if err != nil {
			return err
		}

		if args.Password != "" {
			err = editUser.UpdatePassword(ctx, args.Password)
			if err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return h.usersForm(&editUser, err)(w, r)
	}

	zhttp.Flash(w, T(r.Context(), "notify/users-edited|User ‘%(email)’ edited.", editUser.Email))
	return zhttp.SeeOther(w, "/settings/users")
}

func (h settings) usersRemove(w http.ResponseWriter, r *http.Request) error {
	v := goatcounter.NewValidate(r.Context())
	id := v.Integer("id", chi.URLParam(r, "id"))
	if v.HasErrors() {
		return v
	}

	account := Account(r.Context())

	var user goatcounter.User
	err := user.ByID(r.Context(), id)
	if err != nil {
		return err
	}

	if user.Site != account.ID {
		return guru.New(404, T(r.Context(), "error/not-found|Not Found"))
	}

	err = user.Delete(r.Context(), false)
	if err != nil {
		return err
	}

	zhttp.Flash(w, T(r.Context(), "notify/user-removed|User ‘%(email)’ removed.", user.Email))
	return zhttp.SeeOther(w, "/settings/users")
}

func (h settings) bosmang(w http.ResponseWriter, r *http.Request) error {
	info, _ := zdb.Info(r.Context())
	return zhttp.Template(w, "settings_server.gohtml", struct {
		Globals
		Uptime   string
		Version  string
		Database string
		Go       string
		GOOS     string
		GOARCH   string
		Race     bool
		Cgo      bool
	}{newGlobals(w, r),
		ztime.Now().Sub(Started).Round(time.Second).String(),
		goatcounter.Version,
		zdb.SQLDialect(r.Context()).String() + " " + string(info.Version),
		runtime.Version(),
		runtime.GOOS,
		runtime.GOARCH,
		zruntime.Race,
		zruntime.CGO,
	})
}

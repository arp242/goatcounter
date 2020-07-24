// Copyright © 2019 Martin Tournoij – This file is part of GoatCounter and
// published under the terms of a slightly modified EUPL v1.2 license, which can
// be found in the LICENSE file or at https://license.goatcounter.com

package handlers

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
	"zgo.at/errors"
	"zgo.at/goatcounter"
	"zgo.at/goatcounter/bgrun"
	"zgo.at/guru"
	"zgo.at/zdb"
	"zgo.at/zhttp"
	"zgo.at/zhttp/header"
	"zgo.at/zvalidate"
)

type api struct{}

func (h api) mount(r chi.Router, db zdb.DB) {
	a := r.With(
		middleware.AllowContentType("application/json"),
		zhttp.Ratelimit(zhttp.RatelimitOptions{
			Client: zhttp.RatelimitIP,
			Store:  zhttp.NewRatelimitMemory(),
			Limit:  zhttp.RatelimitLimit(60, 120),
		}))

	//a.Get("/api/v0/count", zhttp.Wrap(h.count))
	a.Post("/api/v0/export", zhttp.Wrap(h.export))
	a.Get("/api/v0/export/{id}", zhttp.Wrap(h.exportGet))
	a.Get("/api/v0/export/{id}/download", zhttp.Wrap(h.exportDownload))

	a.Get("/api/v0/test", zhttp.Wrap(h.test))
	a.Post("/api/v0/test", zhttp.Wrap(h.test))
}

func (h api) auth(r *http.Request, perm goatcounter.APITokenPermissions) error {
	auth := r.Header.Get("Authorization")
	if auth == "" {
		return guru.New(http.StatusForbidden, "no Authorization header")
	}

	b := strings.Fields(auth)
	if len(b) != 2 || b[0] != "Bearer" {
		return guru.New(http.StatusForbidden, "wrong format for Authorization header")
	}

	var token goatcounter.APIToken
	err := token.ByToken(r.Context(), b[1])
	if zdb.ErrNoRows(err) {
		return guru.New(http.StatusForbidden, "unknown token")
	}
	if err != nil {
		return err
	}

	var user goatcounter.User
	err = user.BySite(r.Context(), token.SiteID)
	if err != nil {
		return err
	}

	*r = *r.WithContext(goatcounter.WithUser(r.Context(), &user))

	var need []string
	if perm.Count && !token.Permissions.Count {
		need = append(need, "count")
	}
	if perm.Export && !token.Permissions.Export {
		need = append(need, "export")
	}

	if len(need) > 0 {
		return guru.Errorf(http.StatusForbidden, "requires %s permissions", need)
	}

	return nil
}

type apiExportRequest struct {
	// Pagination cursor; only export hits with an ID greater than this.
	StartFromHitID int64 `json:"start_from_hit_id"`
}

// For testing various generic properties about the API.
func (h api) test(w http.ResponseWriter, r *http.Request) error {
	var args struct {
		Perm     goatcounter.APITokenPermissions `json:"perm"`
		Status   int                             `json:"status"`
		Panic    bool                            `json:"panic"`
		Validate zvalidate.Validator             `json:"validate"`
	}

	_, err := zhttp.Decode(r, &args)
	if err != nil {
		return err
	}

	err = h.auth(r, args.Perm)
	if err != nil {
		return err
	}

	if args.Panic {
		panic("PANIC!")
	}

	if args.Validate.HasErrors() {
		return args.Validate
	}

	if args.Status != 0 {
		w.WriteHeader(args.Status)
		if args.Status == 500 {
			return errors.New("oh noes!")
		}
		return guru.Errorf(args.Status, "status %d", args.Status)
	}

	return zhttp.JSON(w, args)
}

// POST /api/v0/export export
// Start a new export in the background.
//
// This starts a new export in the background.
//
// Request body: apiExportRequest
// Response 202: zgo.at/goatcounter.Export
func (h api) export(w http.ResponseWriter, r *http.Request) error {
	err := h.auth(r, goatcounter.APITokenPermissions{
		Export: true,
	})
	if err != nil {
		return err
	}

	var req apiExportRequest
	_, err = zhttp.Decode(r, &req)
	if err != nil {
		return err
	}

	var export goatcounter.Export
	fp, err := export.Create(r.Context(), req.StartFromHitID)
	if err != nil {
		return err
	}

	ctx := goatcounter.NewContext(r.Context())
	bgrun.Run(fmt.Sprintf("export api:%d", export.SiteID), func() { export.Run(ctx, fp, false) })

	w.WriteHeader(http.StatusAccepted)
	return zhttp.JSON(w, export)
}

// GET /api/v0/export/{id} export
// Get details about an export.
//
// Response 200: zgo.at/goatcounter.Export
func (h api) exportGet(w http.ResponseWriter, r *http.Request) error {
	err := h.auth(r, goatcounter.APITokenPermissions{
		Export: true,
	})
	if err != nil {
		return err
	}

	v := zvalidate.New()
	id := v.Integer("id", chi.URLParam(r, "id"))
	if v.HasErrors() {
		return v
	}

	var export goatcounter.Export
	err = export.ByID(r.Context(), id)
	if err != nil {
		return err
	}

	return zhttp.JSON(w, export)
}

// GET /api/v0/export/{id}/download export
// Download an export file.
//
// Response 200 (text/csv): {data}
func (h api) exportDownload(w http.ResponseWriter, r *http.Request) error {
	err := h.auth(r, goatcounter.APITokenPermissions{
		Export: true,
	})
	if err != nil {
		return err
	}

	v := zvalidate.New()
	id := v.Integer("id", chi.URLParam(r, "id"))
	if v.HasErrors() {
		return v
	}

	var export goatcounter.Export
	err = export.ByID(r.Context(), id)
	if err != nil {
		return err
	}

	fp, err := os.Open(export.Path)
	if err != nil {
		if os.IsNotExist(err) {
			zhttp.FlashError(w, "It looks like there is no export yet.")
			return zhttp.SeeOther(w, "/settings#tab-export")
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

func (h api) count(w http.ResponseWriter, r *http.Request) error {
	err := h.auth(r, goatcounter.APITokenPermissions{
		Count: true,
	})
	if err != nil {
		return err
	}

	return nil
}

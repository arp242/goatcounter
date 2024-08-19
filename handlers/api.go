// Copyright © Martin Tournoij – This file is part of GoatCounter and published
// under the terms of a slightly modified EUPL v1.2 license, which can be found
// in the LICENSE file or at https://license.goatcounter.com

package handlers

import (
	"context"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"zgo.at/bgrun"
	"zgo.at/errors"
	"zgo.at/goatcounter/v2"
	"zgo.at/goatcounter/v2/cron"
	"zgo.at/goatcounter/v2/metrics"
	"zgo.at/guru"
	"zgo.at/isbot"
	"zgo.at/zdb"
	"zgo.at/zhttp"
	"zgo.at/zhttp/header"
	"zgo.at/zhttp/mware"
	"zgo.at/zlog"
	"zgo.at/zstd/zbool"
	"zgo.at/zstd/zint"
	"zgo.at/zstd/zslice"
	"zgo.at/zstd/ztime"
	"zgo.at/zvalidate"
)

type (
	// Generic API error. An error will have either the "error" or "errors"
	// field set, but not both.
	apiError struct {
		Error  string              `json:"error,omitempty"`
		Errors map[string][]string `json:"errors,omitempty"`
	}
	// Authentication error: the API key was not provided or incorrect.
	//lint:ignore U1000 referenced from Kommentaar.
	authError struct {
		Error string `json:"error,omitempty"`
	}
)

const respOK = `{"status":"ok"}`

type api struct {
	apiMax, apiMaxPaths int
	dec                 zhttp.Decoder
}

func newAPI(apiMax int) api {
	return api{apiMax: apiMax, dec: zhttp.NewDecoder(false, true)}
}

func (h api) mount(r chi.Router, db zdb.DB) {
	h.apiMaxPaths = h.apiMax
	if h.apiMax == 0 {
		h.apiMax = 100
	}
	if h.apiMaxPaths == 0 {
		h.apiMaxPaths = 200
	}

	a := r.With(
		middleware.AllowContentType("application/json"),
		mware.Ratelimit(mware.RatelimitOptions{
			Client: mware.RatelimitIP,
			Store:  mware.NewRatelimitMemory(),
			Limit: func(r *http.Request) (int, int64) {
				switch r.URL.Path {
				default:
					return rateLimits.api(r)
				case "/api/v0/export":
					return rateLimits.export(r)
				case "/api/v0/count":
					return rateLimits.apiCount(r)
				}
			},
		}),
	)

	a.Get("/api/v0/test", zhttp.Wrap(h.test))
	a.Post("/api/v0/test", zhttp.Wrap(h.test))

	a.Get("/api/v0/me", zhttp.Wrap(h.me))

	a.Post("/api/v0/export", zhttp.Wrap(h.export))
	a.Get("/api/v0/export/{id}", zhttp.Wrap(h.exportGet))
	a.Get("/api/v0/export/{id}/download", zhttp.Wrap(h.exportDownload))

	a.Post("/api/v0/count", zhttp.Wrap(h.count))

	a.Get("/api/v0/paths", zhttp.Wrap(h.paths))
	a.Get("/api/v0/stats/total", zhttp.Wrap(h.countTotal))
	a.Get("/api/v0/stats/hits", zhttp.Wrap(h.hits))
	a.Get("/api/v0/stats/hits/{path_id}", zhttp.Wrap(h.refs))
	a.Get("/api/v0/stats/{page}", zhttp.Wrap(h.stats))
	a.Get("/api/v0/stats/{page}/{id}", zhttp.Wrap(h.statsDetail))

	// Note: DELETE not supported for sites and users intentionally, since it's
	// such a dangerous operation.
	a.Get("/api/v0/sites", zhttp.Wrap(h.siteList))
	a.Put("/api/v0/sites", zhttp.Wrap(h.siteCreate))
	a.Get("/api/v0/sites/{id}", zhttp.Wrap(h.siteGet))
	a.Post("/api/v0/sites/{id}", zhttp.Wrap(h.siteUpdate))  // Update all
	a.Patch("/api/v0/sites/{id}", zhttp.Wrap(h.siteUpdate)) // Update just fields given
}

func tokenFromHeader(r *http.Request, w http.ResponseWriter) (string, error) {
	auth := r.Header.Get("Authorization")
	if auth == "" {
		return "", guru.New(http.StatusUnauthorized, "no Authorization header")
	}

	b := strings.Fields(auth)
	if len(b) != 2 {
		return "", guru.New(http.StatusUnauthorized, "wrong format for Authorization header")
	}
	switch b[0] {
	case "Bearer":
		return b[1], nil
	case "Basic":
		k, err := base64.StdEncoding.DecodeString(b[1])
		if err != nil {
			return "", guru.Wrap(http.StatusUnauthorized, err, "wrong format for Authorization header")
		}
		_, key, ok := strings.Cut(string(k), ":")
		if !ok || key == "" {
			return "", guru.Wrap(http.StatusUnauthorized, err, "wrong format for Authorization header")
		}
		return key, nil
	default:
		return "", guru.New(http.StatusUnauthorized, "wrong format for Authorization header")
	}
}

var (
	bufferKeyOnce sync.Once
	bufferKey     []byte
)

// ResetBufferKey resets the buffer key, for tests.
func ResetBufferKey() {
	bufferKeyOnce = sync.Once{}
	bufferKey = []byte{}
}

func (h api) auth(r *http.Request, w http.ResponseWriter, require zint.Bitflag64) error {
	key, err := tokenFromHeader(r, w)
	if err != nil {
		w.Header().Set("WWW-Authenticate", "Basic realm=GoatCounter")
		return err
	}

	// From "goatcounter buffer".
	if r.Header.Get("X-Goatcounter-Buffer") == "1" && r.URL.Path == "/api/v0/count" {
		bufferKeyOnce.Do(func() {
			bufferKey, err = goatcounter.LoadBufferKey(r.Context())
			if err != nil {
				zlog.Error(err)
			}
		})
		if subtle.ConstantTimeCompare(bufferKey, []byte(key)) == 0 {
			return guru.New(http.StatusUnauthorized, "unknown buffer token")
		}
		return nil
	}

	// Regular API token.
	var token goatcounter.APIToken
	err = token.ByToken(r.Context(), key)
	if zdb.ErrNoRows(err) {
		w.Header().Set("WWW-Authenticate", "Basic realm=GoatCounter")
		return guru.New(http.StatusUnauthorized, "unknown token")
	}
	if err != nil {
		return err
	}

	// Update once a day at the most.
	if token.LastUsedAt == nil || token.LastUsedAt.Before(ztime.Now().Add(-24*time.Hour)) {
		err := token.UpdateLastUsed(r.Context())
		if err != nil {
			zlog.Error(err)
		}
	}

	var user goatcounter.User
	err = user.ByID(r.Context(), token.UserID)
	if err != nil {
		return err
	}

	// API is only for admins at the moment; other users shouldn't be able to
	// create an API key, but just in case.
	if !user.AccessAdmin() {
		return guru.New(401, "only admins can create and use API keys")
	}

	*r = *r.WithContext(goatcounter.WithUser(r.Context(), &user))

	if require == 0 {
		return nil
	}
	if !token.Permissions.Has(require) {
		return guru.Errorf(http.StatusForbidden, "requires %s permissions",
			goatcounter.APIToken{Permissions: require}.FormatPermissions())
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
		Perm     zint.Bitflag64      `json:"perm"`
		Status   int                 `json:"status"`
		Panic    bool                `json:"panic"`
		Validate zvalidate.Validator `json:"validate"`
		Context  bool                `json:"context"`
	}

	_, err := h.dec.Decode(r, &args)
	if err != nil {
		return err
	}

	err = h.auth(r, w, args.Perm)
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

	if args.Context {
		info, _ := zdb.Info(r.Context())
		return zhttp.JSON(w, map[string]any{
			"site_id": Site(r.Context()).ID,
			"serve":   !goatcounter.Config(r.Context()).GoatcounterCom,
			"db":      info.Version,
		})
	}

	return zhttp.JSON(w, args)
}

type meResponse struct {
	User  goatcounter.User     `json:"user"`
	Token goatcounter.APIToken `json:"token"`
}

// GET /api/v0/me users
// Get information about the current user and API key.
//
// Response 200: meResponse
func (h api) me(w http.ResponseWriter, r *http.Request) error {
	err := h.auth(r, w, 0)
	if err != nil {
		return err
	}

	key, err := tokenFromHeader(r, w)
	if err != nil {
		return err
	}
	var token goatcounter.APIToken
	err = token.ByToken(r.Context(), key)
	if err != nil {
		return err
	}

	u := User(r.Context())
	return zhttp.JSON(w, meResponse{User: *u, Token: token})
}

// POST /api/v0/export export
// Start a new export in the background.
//
// This starts a new export in the background; this can only be done once an
// hour.
//
// Request body: apiExportRequest
// Response 202: zgo.at/goatcounter/v2.Export
func (h api) export(w http.ResponseWriter, r *http.Request) error {
	err := h.auth(r, w, goatcounter.APIPermExport)
	if err != nil {
		return err
	}

	var req apiExportRequest
	_, err = h.dec.Decode(r, &req)
	if err != nil {
		return err
	}

	var export goatcounter.Export
	fp, err := export.Create(r.Context(), req.StartFromHitID)
	if err != nil {
		return err
	}

	ctx := goatcounter.CopyContextValues(r.Context())
	bgrun.MustRunFunction(fmt.Sprintf("export api:%d", export.SiteID), func() { export.Run(ctx, fp, false) })

	w.WriteHeader(http.StatusAccepted)
	return zhttp.JSON(w, export)
}

// GET /api/v0/export/{id} export
// Get details about an export.
//
// Response 200: zgo.at/goatcounter/v2.Export
func (h api) exportGet(w http.ResponseWriter, r *http.Request) error {
	err := h.auth(r, w, goatcounter.APIPermExport)
	if err != nil {
		return err
	}

	v := goatcounter.NewValidate(r.Context())
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
// The export may take a while to generate, depending on the size. It will
// return a 202 Accepted status code if the export ID is still running.
//
// Export files are kept for 24 hours, after which they're deleted. This will
// return a 400 Gone status code if the export has been deleted.
//
// Response 200 (text/csv): {data}
// Response 202: zgo.at/goatcounter/v2/handlers.apiError
// Response 400: zgo.at/goatcounter/v2/handlers.apiError
func (h api) exportDownload(w http.ResponseWriter, r *http.Request) error {
	err := h.auth(r, w, goatcounter.APIPermExport)
	if err != nil {
		return err
	}

	v := goatcounter.NewValidate(r.Context())
	id := v.Integer("id", chi.URLParam(r, "id"))
	if v.HasErrors() {
		return v
	}

	var export goatcounter.Export
	err = export.ByID(r.Context(), id)
	if err != nil {
		return err
	}

	if export.FinishedAt == nil {
		w.WriteHeader(202)
		return zhttp.JSON(w, apiError{Error: "still being generated"})
	}

	fp, err := os.Open(export.Path)
	if err != nil {
		if os.IsNotExist(err) && export.FinishedAt.Add(24*time.Hour).After(ztime.Now()) {
			w.WriteHeader(400)
			return zhttp.JSON(w, apiError{Error: "exports are kept for 24 hours; this export file has been deleted"})
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

type APICountRequest struct {
	// By default it's an error to send pageviews that don't have either a
	// Session or UserAgent and IP set. This avoids accidental errors.
	//
	// When this is set it will just continue without recording sessions for
	// pageviews that don't have these parameters set.
	NoSessions bool `json:"no_sessions"`

	// Filter pageviews; accepted values:
	//
	//   ip     Ignore requests coming from IP addresses listed in "Settings → Ignore IP". Requires the IP field to be set.
	//
	// ["ip"] is used if this field isn't sent; send an empty array ([]) to not
	// filter anything.
	//
	// The X-Goatcounter-Filter header will be set to a list of indexes if any
	// pageviews are filtered; for example:
	//
	//    X-Goatcounter-Filter: 5, 10
	//
	// This header will be omitted if nothing is filtered.
	Filter []string `json:"filter"`

	// Hits is the list of pageviews.
	Hits []APICountRequestHit `json:"hits"`
}

type APICountRequestHit struct {
	// Path of the pageview, or the event name. {required}
	Path string `json:"path" query:"p"`

	// Page title, or some descriptive event title.
	Title string `json:"title" query:"t"`

	// Is this an event?
	Event zbool.Bool `json:"event" query:"e"`

	// Referrer value, can be an URL (i.e. the Referal: header) or any
	// string.
	Ref string `json:"ref" query:"r"`

	// Screen size as "x,y,scaling"
	Size goatcounter.Floats `json:"size" query:"s"`

	// Query parameters for this pageview, used to get campaign parameters.
	Query string `json:"query" query:"q"`

	// Hint if this should be considered a bot; should be one of the JSBot*`
	// constants from isbot; note the backend may override this if it
	// detects a bot using another method.
	// https://github.com/zgoat/isbot/blob/master/isbot.go#L28
	Bot int `json:"bot" query:"b"`

	// User-Agent header.
	UserAgent string `json:"user_agent"`

	// Location as ISO-3166-1 alpha2 string (e.g. NL, ID, etc.)
	Location string `json:"location"`

	// IP to get location from; not used if location is set. Also used for
	// session generation.
	IP string `json:"ip"`

	// Time this pageview should be recorded at; this can be in the past,
	// but not in the future.
	CreatedAt time.Time `json:"created_at"`

	// Normally a session is based on hash(User-Agent+IP+salt), but if you don't
	// send the IP address then we can't determine the session.
	//
	// In those cases, you can store your own session identifiers and send them
	// along. Note these will not be stored in the database as the sessionID
	// (just as the hashes aren't), they're just used as a unique grouping
	// identifier.
	Session string `json:"session"`

	// {omitdoc}
	Host string `json:"-"`

	// {omitdoc} Line when importing, for displaying errors.
	Line string `json:"-"`
	// {omitdoc} Line when importing, for displaying errors.
	LineNo uint64 `json:"-"`
}

func (h APICountRequestHit) String() string {
	return fmt.Sprintf(
		`{Path: %q, Title: %q, Event: %t, Ref: %q, Size: "%s", Query: %q, Bot: %d, UserAgent: %q, Location: %q, IP: %q, CreatedAt: %q, Session: %q, Host: %q}`,
		h.Path, h.Title, h.Event, h.Ref, h.Size, h.Query, h.Bot, h.UserAgent, h.Location, h.IP, h.CreatedAt, h.Session, h.Host)
}

// POST /api/v0/count count
// Count pageviews.
//
// This can count one or more pageviews. Pageviews are not persisted
// immediately, but persisted in the background every 10 seconds.
//
// The maximum amount of pageviews per request is 500.
//
// Errors will have the key set to the index of the pageview. Any pageviews not
// listed have been processed and shouldn't be sent again.
//
// Request body: APICountRequest
// Response 202: {empty}
func (h api) count(w http.ResponseWriter, r *http.Request) error {
	m := metrics.Start("/api/v0/count")
	defer m.Done()

	err := h.auth(r, w, goatcounter.APIPermCount)
	if err != nil {
		return err
	}

	var args APICountRequest
	_, err = h.dec.Decode(r, &args)
	if err != nil {
		return err
	}

	if len(args.Hits) == 0 {
		w.WriteHeader(400)
		return zhttp.JSON(w, apiError{Error: "no hits"})
	}
	if len(args.Hits) > 500 {
		w.WriteHeader(400)
		return zhttp.JSON(w, apiError{Error: "maximum amount of pageviews in one batch is 500"})
	}
	if args.Filter == nil {
		args.Filter = []string{"ip"}
	}
	filterIP := zslice.Remove(&args.Filter, "ip")
	if len(args.Filter) > 0 {
		return zhttp.JSON(w, apiError{Error: fmt.Sprintf("unknown value in Filter: %v", args.Filter)})
	}

	var (
		errs       = make(map[int]string)
		filter     []int
		site       = Site(r.Context())
		firstHitAt = site.FirstHitAt
	)
	for i, a := range args.Hits {
		if filterIP && a.IP != "" && slices.Contains(site.Settings.IgnoreIPs, a.IP) {
			filter = append(filter, i)
			continue
		}

		if a.Location == "" && a.IP != "" {
			a.Location = (goatcounter.Location{}).LookupIP(r.Context(), a.IP)
		}

		hit := goatcounter.Hit{
			Path:            a.Path,
			Title:           a.Title,
			Ref:             a.Ref,
			Event:           a.Event,
			Size:            a.Size,
			Query:           a.Query,
			Bot:             a.Bot,
			CreatedAt:       a.CreatedAt.UTC(),
			UserAgentHeader: a.UserAgent,
			Location:        a.Location,
			RemoteAddr:      a.IP,
		}

		if a.UserAgent != "" {
			if b := isbot.UserAgent(a.UserAgent); isbot.Is(b) {
				hit.Bot = int(b)
			}
		}

		switch {
		case a.Session != "":
			hit.UserSessionID = a.Session
		case hit.UserAgentHeader != "" && a.IP != "":
			// Handle as usual in memstore.
		case !args.NoSessions:
			errs[i] = "session or browser/IP not set; use no_sessions if you don't want to track unique visits"
			continue
		}

		hit.Defaults(r.Context(), true) // don't get UA/Path; memstore will do that.
		err = hit.Validate(r.Context(), true)
		if err != nil {
			errs[i] = err.Error()
			continue
		}

		if hit.CreatedAt.Before(firstHitAt) {
			firstHitAt = hit.CreatedAt
		}
		goatcounter.Memstore.Append(hit)
	}

	if len(filter) > 0 {
		w.Header().Set("X-Goatcounter-Filter", zint.Join(filter, ", "))
	}
	if len(errs) > 0 {
		w.WriteHeader(400)
		return zhttp.JSON(w, map[string]any{
			"errors": errs,
		})
	}

	if goatcounter.Memstore.Len() >= 5000 {
		cron.WaitPersistAndStat()
		err := cron.TaskPersistAndStat()
		if err != nil {
			zlog.Error(err)
		}
	}

	if !firstHitAt.Equal(site.FirstHitAt) {
		err := site.UpdateFirstHitAt(r.Context(), firstHitAt)
		if err != nil {
			zlog.Module("api-import").Fields(zlog.F{
				"site":       site.ID,
				"firstHitAt": firstHitAt.String(),
			}).Error(err)
		}
	}

	w.WriteHeader(http.StatusAccepted)
	return zhttp.JSON(w, respOK)
}

type apiSitesResponse struct {
	Sites goatcounter.Sites `json:"sites"`
}

// GET /api/v0/sites sites
// List all sites.
//
// Response 200: apiSitesResponse
func (h api) siteList(w http.ResponseWriter, r *http.Request) error {
	err := h.auth(r, w, goatcounter.APIPermSiteRead)
	if err != nil {
		return err
	}

	sites := goatcounter.Sites{*goatcounter.MustGetSite(r.Context())}
	err = sites.ListSubs(r.Context())
	if err != nil {
		return err
	}

	return zhttp.JSON(w, apiSitesResponse{sites})
}

func (h api) siteFind(r *http.Request) (*goatcounter.Site, error) {
	v := goatcounter.NewValidate(r.Context())
	id := v.Integer("id", chi.URLParam(r, "id"))
	if v.HasErrors() {
		return nil, v
	}

	var site goatcounter.Site
	err := site.ByID(r.Context(), id)
	if err != nil {
		return nil, err
	}

	siteID := Site(r.Context()).ID
	if !(site.ID == siteID || (site.Parent != nil && *site.Parent == siteID)) {
		return nil, guru.New(404, "")
	}

	return &site, nil
}

// GET /api/v0/sites/{id} sites
// Get information about a site.
//
// Get all information about one site.
//
// Response 200: goatcounter.Site
func (h api) siteGet(w http.ResponseWriter, r *http.Request) error {
	err := h.auth(r, w, goatcounter.APIPermSiteRead)
	if err != nil {
		return err
	}

	site, err := h.siteFind(r)
	if err != nil {
		return err
	}
	return zhttp.JSON(w, site)
}

// PUT /api/v0/sites sites
// Create a new site.
//
// Request body: goatcounter.Site
// Response 200: goatcounter.Site
func (h api) siteCreate(w http.ResponseWriter, r *http.Request) error {
	err := h.auth(r, w, goatcounter.APIPermSiteCreate)
	if err != nil {
		return err
	}

	var site goatcounter.Site
	_, err = h.dec.Decode(r, &site)
	if err != nil {
		return err
	}

	site.Parent = &Site(r.Context()).ID
	err = site.Insert(r.Context())
	if err != nil {
		return err
	}

	return zhttp.JSON(w, site)
}

type apiSiteUpdateRequest struct {
	Settings   goatcounter.SiteSettings `json:"settings"`
	Cname      *string                  `json:"cname"`
	LinkDomain string                   `json:"link_domain"`
}

// POST /api/v0/sites/{id} sites
// PATCH /api/v0/sites/{id} sites
// Update a site.
//
// A POST request will *replace* the entire site with what's sent, blanking out
// any existing fields that may exist. A PATCH request will only update the
// fields that are sent.
//
// Request body: apiSiteUpdateRequest
// Response 200: goatcounter.Site
func (h api) siteUpdate(w http.ResponseWriter, r *http.Request) error {
	err := h.auth(r, w, goatcounter.APIPermSiteUpdate)
	if err != nil {
		return err
	}

	site, err := h.siteFind(r)
	if err != nil {
		return err
	}

	var args apiSiteUpdateRequest
	if r.Method == http.MethodPatch {
		args.LinkDomain = site.LinkDomain
		args.Cname = site.Cname
		args.Settings = site.Settings
	}

	_, err = h.dec.Decode(r, &args)
	if err != nil {
		return err
	}

	site.LinkDomain = args.LinkDomain
	site.Cname = args.Cname
	site.Settings = args.Settings
	err = site.Update(r.Context())
	if err != nil {
		return err
	}

	return zhttp.JSON(w, site)
}

type (
	apiPathsRequest struct {
		// Limit number of returned results {range: 1-200, default: 20}
		Limit int `json:"limit"`

		// Only select paths after this ID, for pagination.
		After int64 `json:"after"`
	}
	apiPathsResponse struct {
		// List of paths, sorted by ID.
		Paths goatcounter.Paths `json:"paths"`

		// True if there are more paths.
		More bool `json:"more"`
	}
)

// GET /api/v0/paths paths
// Get an overview of paths on this site (without statistics).
//
// Query: apiPathsRequest
// Response 200: apiPathsResponse
func (h api) paths(w http.ResponseWriter, r *http.Request) error {
	m := metrics.Start("/api/v0/stats/*")
	defer m.Done()

	err := h.auth(r, w, goatcounter.APIPermStats)
	if err != nil {
		return err
	}

	args := apiPathsRequest{Limit: 20}
	if _, err := h.dec.Decode(r, &args); err != nil {
		return err
	}

	if h.apiMaxPaths > 0 && args.Limit > h.apiMaxPaths {
		args.Limit = h.apiMaxPaths
	}
	if args.Limit < 1 {
		args.Limit = 1
	}

	var p goatcounter.Paths
	more, err := p.List(r.Context(), goatcounter.MustGetSite(r.Context()).ID, args.After, args.Limit)
	if err != nil {
		return err
	}
	return zhttp.JSON(w, apiPathsResponse{Paths: p, More: more})
}

type (
	apiHitsRequest struct {
		// Start time, should be rounded to the hour {datetime, default: one week ago}.
		Start time.Time `json:"start" query:"start"`

		// End time, should be rounded to the hour {datetime, default: current time}.
		End time.Time `json:"end" query:"end"`

		// Group by day, rather than by hour. This only affects the Hits.Max
		// value: if enabled it's set to the highest value for that day, rather
		// than the highest value for the hour.
		Daily bool `json:"daily" query:"daily"`

		// Include only these paths; default is to include everything.
		IncludePaths goatcounter.Ints `json:"include_paths" query:"include_paths"`

		// Exclude these paths, for pagination.
		ExcludePaths goatcounter.Ints `json:"exclude_paths" query:"exclude_paths"`

		// Maximum number of pages to get {range: 1-100, default: 20}.
		Limit int `json:"limit" query:"limit"`
	}
	apiHitsResponse struct {
		// Sorted list of paths with their visitor and pageview count.
		Hits goatcounter.HitLists `json:"hits"`

		// Total number of visitors in the returned result.
		Total int `json:"total"`

		// More hits after this?
		More bool `json:"more"`
	}
)

// GET /api/v0/stats/hits stats
// Get an overview of pageviews.
//
// Query: apiHitsRequest
// Response 200: apiHitsResponse
func (h api) hits(w http.ResponseWriter, r *http.Request) error {
	m := metrics.Start("/api/v0/stats/*")
	defer m.Done()

	err := h.auth(r, w, goatcounter.APIPermStats)
	if err != nil {
		return err
	}

	args := apiHitsRequest{Limit: 20}
	if _, err := h.dec.Decode(r, &args); err != nil {
		return err
	}
	if h.apiMax > 0 && args.Limit > h.apiMax {
		args.Limit = h.apiMax
	}
	if args.Limit < 1 {
		args.Limit = 1
	}
	if args.Start.IsZero() {
		args.Start = ztime.AddPeriod(ztime.Now(), -7, ztime.Day)
	}
	if args.End.IsZero() {
		args.End = ztime.Now()
	}

	var pages goatcounter.HitLists
	tdu, more, err := pages.List(r.Context(), ztime.NewRange(args.Start).To(args.End),
		args.IncludePaths, args.ExcludePaths, args.Limit, args.Daily)
	if err != nil {
		return err
	}

	return zhttp.JSON(w, apiHitsResponse{
		Total: tdu,
		Hits:  pages,
		More:  more,
	})
}

type (
	apiRefsRequest struct {
		// Start time, should be rounded to the hour {datetime, default: one week ago}.
		Start time.Time `json:"start" query:"start"`

		// End time, should be rounded to the hour {datetime, default: current time}.
		End time.Time `json:"end" query:"end"`

		// Maximum number of pages to get {range: 1-100, default: 20}.
		Limit int `json:"limit" query:"limit"`

		// Offset for pagination.
		Offset int `json:"offset" query:"offset"`
	}
	apiRefsResponse struct {
		Refs []goatcounter.HitStat `json:"refs"`
		More bool                  `json:"more"`
	}
)

// GET /api/v0/stats/hits/{path_id} stats
// Get an overview of referral information for a path.
//
// Query: apiRefsRequest
// Response 200: apiRefsResponse
func (h api) refs(w http.ResponseWriter, r *http.Request) error {
	m := metrics.Start("/api/v0/stats/*")
	defer m.Done()

	err := h.auth(r, w, goatcounter.APIPermStats)
	if err != nil {
		return err
	}

	v := zvalidate.New()
	path := v.Integer("path_id", chi.URLParam(r, "path_id"))
	if v.HasErrors() {
		return v
	}

	args := apiRefsRequest{Limit: 20}
	if _, err := h.dec.Decode(r, &args); err != nil {
		return err
	}
	if h.apiMax > 0 && args.Limit > h.apiMax {
		args.Limit = h.apiMax
	}
	if args.Limit < 1 {
		args.Limit = 1
	}
	if args.Start.IsZero() {
		args.Start = ztime.AddPeriod(ztime.Now(), -7, ztime.Day)
	}
	if args.End.IsZero() {
		args.End = ztime.Now()
	}

	var refs goatcounter.HitStats
	err = refs.ListRefsByPathID(r.Context(), path, ztime.NewRange(args.Start).To(args.End),
		args.Limit, args.Offset)
	if err != nil {
		return err
	}

	return zhttp.JSON(w, apiRefsResponse{
		Refs: refs.Stats,
		More: refs.More,
	})
}

type (
	apiCountTotalRequest struct {
		// Start time, should be rounded to the hour {datetime, default: one week ago}.
		Start time.Time `json:"start" query:"start"`

		// End time, should be rounded to the hour {datetime, default: current time}.
		End time.Time `json:"end" query:"end"`

		// Include only these paths; default is to include everything.
		IncludePaths goatcounter.Ints `json:"include_paths" query:"include_paths"`
	}
)

// GET /api/v0/stats/total stats
// Count total number of pageviews for a date range.
//
// This is mostly useful to display things like browser stats as a percentage of
// the total; the /api/v0/pages endpoint only counts the pageviews until it's
// paginated.
//
// Query: apiCountTotalRequest
// Response 200: goatcounter.TotalCount
func (h api) countTotal(w http.ResponseWriter, r *http.Request) error {
	m := metrics.Start("/api/v0/stats/*")
	defer m.Done()

	err := h.auth(r, w, goatcounter.APIPermStats)
	if err != nil {
		return err
	}

	var args apiCountTotalRequest
	if _, err := h.dec.Decode(r, &args); err != nil {
		return err
	}
	if args.Start.IsZero() {
		args.Start = ztime.AddPeriod(ztime.Now(), -7, ztime.Day)
	}
	if args.End.IsZero() {
		args.End = ztime.Now()
	}

	tc, err := goatcounter.GetTotalCount(r.Context(), ztime.NewRange(args.Start).To(args.End),
		args.IncludePaths, false)
	if err != nil {
		return err
	}

	return zhttp.JSON(w, tc)
}

type (
	apiStatsRequest struct {
		// Start time, should be rounded to the hour {datetime, default: one week ago}.
		Start time.Time `json:"start" query:"start"`

		// End time, should be rounded to the hour {datetime, default: current time}.
		End time.Time `json:"end" query:"end"`

		// Include only these paths; default is to include everything.
		IncludePaths goatcounter.Ints `json:"include_paths" query:"include_paths"`

		// Maximum number of pages to get {range: 1-100, default: 20}.
		Limit int `json:"limit" query:"limit"`

		// Offset for pagination.
		Offset int `json:"offset" query:"offset"`
	}
	apiStatsResponse struct {
		// Sorted list of paths with their visitor and pageview count.
		Stats []goatcounter.HitStat `json:"stats"`
		More  bool                  `json:"more"`
	}
)

// GET /api/v0/stats/{page} stats
// Get browser/system/etc. stats.
//
// Page can be: browsers, systems, locations, languages, sizes, campaigns,
// toprefs.
//
// Query: apiStatsRequest
// Response 200: apiStatsResponse
func (h api) stats(w http.ResponseWriter, r *http.Request) error {
	m := metrics.Start("/api/v0/stats/*")
	defer m.Done()

	v := goatcounter.NewValidate(r.Context())
	page := v.Include("page", chi.URLParam(r, "page"), []string{
		"browsers", "systems", "locations", "languages", "sizes", "campaigns", "toprefs"})
	if v.HasErrors() {
		return v
	}

	err := h.auth(r, w, goatcounter.APIPermStats)
	if err != nil {
		return err
	}

	args := apiStatsRequest{Limit: 20}
	if _, err := h.dec.Decode(r, &args); err != nil {
		return err
	}
	if h.apiMax > 0 && args.Limit > h.apiMax {
		args.Limit = h.apiMax
	}
	if args.Limit < 1 {
		args.Limit = 1
	}
	if args.Start.IsZero() {
		args.Start = ztime.AddPeriod(ztime.Now(), -7, ztime.Day)
	}
	if args.End.IsZero() {
		args.End = ztime.Now()
	}

	var (
		stats goatcounter.HitStats
		f     func(ctx context.Context, rng ztime.Range, pathFilter []int64, limit, offset int) error
	)
	switch page {
	case "browsers":
		f = stats.ListBrowsers
	case "systems":
		f = stats.ListSystems
	case "locations":
		f = stats.ListLocations
	case "languages":
		f = stats.ListLanguages
	case "sizes":
		f = func(ctx context.Context, rng ztime.Range, pathFilter []int64, _, _ int) error {
			return stats.ListSizes(ctx, rng, pathFilter)
		}
	case "campaigns":
		f = stats.ListCampaigns
	case "toprefs":
		f = stats.ListTopRefs
	}
	err = f(r.Context(), ztime.NewRange(args.Start).To(args.End), args.IncludePaths, args.Limit, args.Offset)
	if err != nil {
		return err
	}

	// Name is used as ID for some; setting it here makes for a nicer API.
	// TODO: should probably use the "real" ID now that we have tables for that.
	for i := range stats.Stats {
		if stats.Stats[i].ID == "" {
			stats.Stats[i].ID = stats.Stats[i].Name
		}
	}

	return zhttp.JSON(w, apiStatsResponse{
		Stats: stats.Stats,
		More:  stats.More,
	})
}

// GET /api/v0/stats/{page}/{id} stats
// Get detailed stats for an ID.
//
// Page can be: browsers, systems, locations, sizes, campaigns, toprefs.
//
// Query: apiStatsRequest
// Response 200: apiStatsResponse
func (h api) statsDetail(w http.ResponseWriter, r *http.Request) error {
	m := metrics.Start("/api/v0/stats/*")
	defer m.Done()

	v := goatcounter.NewValidate(r.Context())
	page := v.Include("page", chi.URLParam(r, "page"), []string{
		"browsers", "systems", "locations", "sizes", "campaigns", "toprefs"})
	if v.HasErrors() {
		return v
	}

	err := h.auth(r, w, goatcounter.APIPermStats)
	if err != nil {
		return err
	}

	args := apiStatsRequest{Limit: 20}
	if _, err := h.dec.Decode(r, &args); err != nil {
		return err
	}
	if h.apiMax > 0 && args.Limit > h.apiMax {
		args.Limit = h.apiMax
	}
	if args.Limit < 1 {
		args.Limit = 1
	}
	if args.Start.IsZero() {
		args.Start = ztime.AddPeriod(ztime.Now(), -7, ztime.Day)
	}
	if args.End.IsZero() {
		args.End = ztime.Now()
	}

	var (
		stats goatcounter.HitStats
		f     func(ctx context.Context, id string, rng ztime.Range, pathFilter []int64, limit, offset int) error
	)
	switch page {
	case "browsers":
		f = stats.ListBrowser
	case "systems":
		f = stats.ListSystem
	case "locations":
		f = stats.ListLocation
	case "sizes":
		f = stats.ListSize
	case "toprefs":
		f = stats.ListTopRef
	case "campaigns":
		f = func(ctx context.Context, id string, rng ztime.Range, pathFilter []int64, limit, offset int) error {
			n, err := strconv.ParseInt(id, 0, 64)
			if err != nil {
				return err
			}
			return stats.ListCampaign(ctx, n, rng, pathFilter, limit, offset)
		}
	}
	err = f(r.Context(), chi.URLParam(r, "id"), ztime.NewRange(args.Start).To(args.End),
		args.IncludePaths, args.Limit, args.Offset)
	if err != nil {
		return err
	}

	return zhttp.JSON(w, apiStatsResponse{
		Stats: stats.Stats,
		More:  stats.More,
	})
}

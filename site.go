// Copyright © 2019 Martin Tournoij <martin@arp242.net>
// This file is part of GoatCounter and published under the terms of the EUPL
// v1.2, which can be found in the LICENSE file or at http://eupl12.zgo.at

package goatcounter

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"zgo.at/goatcounter/cfg"
	"zgo.at/goatcounter/errors"
	"zgo.at/guru"
	"zgo.at/tz"
	"zgo.at/utils/jsonutil"
	"zgo.at/zdb"
	"zgo.at/zhttp"
	"zgo.at/zlog"
	"zgo.at/zvalidate"
)

// Plan column values.
const (
	PlanPersonal     = "personal"
	PlanPersonalPlus = "personalplus"
	PlanBusiness     = "business"
	PlanBusinessPlus = "businessplus"
	PlanChild        = "child"
)

var Plans = []string{PlanPersonal, PlanPersonalPlus, PlanBusiness, PlanBusinessPlus}

var reserved = []string{
	"www", "mail", "smtp", "imap", "static",
	"admin", "ns1", "ns2", "m", "mobile", "api",
	"dev", "test", "beta", "new", "staging", "debug", "pprof",
	"chat", "example", "yoursite", "test", "sql",
}

var statTables = []string{"hit_stats", "browser_stats", "location_stats", "ref_stats", "size_stats"}

// Site is a single site which is sending newsletters (i.e. it's a "customer").
type Site struct {
	ID     int64  `db:"id"`
	Parent *int64 `db:"parent"`

	Cname        *string      `db:"cname"`       // Custom domain, e.g. "stats.example.com"
	Code         string       `db:"code"`        // Domain code (arp242, which makes arp242.goatcounter.com)
	LinkDomain   string       `db:"link_domain"` // Site domain for linking (www.arp242.net).
	Plan         string       `db:"plan"`
	Stripe       *string      `db:"stripe"`
	Settings     SiteSettings `db:"settings"`
	ReceivedData bool         `db:"received_data"`

	State     string     `db:"state"`
	CreatedAt time.Time  `db:"created_at"`
	UpdatedAt *time.Time `db:"updated_at"`
}

type SiteSettings struct {
	Public           bool        `json:"public"`
	TwentyFourHours  bool        `json:"twenty_four_hours"`
	SundayStartsWeek bool        `json:"sunday_starts_week"`
	DateFormat       string      `json:"date_format"`
	NumberFormat     rune        `json:"number_format"`
	DataRetention    int         `json:"data_retention"`
	IgnoreIPs        zdb.Strings `json:"ignore_ips"`
	Timezone         *tz.Zone    `json:"timezone"`
	Campaigns        zdb.Strings `json:"campaigns"`
	Limits           struct {
		Page int `json:"page"`
		Ref  int `json:"ref"`
	} `json:"limits"`
}

func (ss SiteSettings) String() string { return string(jsonutil.MustMarshal(ss)) }

// Value implements the SQL Value function to determine what to store in the DB.
func (ss SiteSettings) Value() (driver.Value, error) { return json.Marshal(ss) }

// Scan converts the data returned from the DB into the struct.
func (ss *SiteSettings) Scan(v interface{}) error {
	switch vv := v.(type) {
	case []byte:
		return json.Unmarshal(vv, ss)
	case string:
		return json.Unmarshal([]byte(vv), ss)
	default:
		panic(fmt.Sprintf("unsupported type: %T", v))
	}
}

// Defaults sets fields to default values, unless they're already set.
func (s *Site) Defaults(ctx context.Context) {
	// New site: Set default settings.
	if s.ID == 0 {
		s.Settings.Campaigns = []string{"utm_campaign", "utm_source", "ref"}
	}

	if s.State == "" {
		s.State = StateActive
	}

	if s.Settings.DateFormat == "" {
		s.Settings.DateFormat = "2 Jan ’06"
	}
	if s.Settings.NumberFormat == 0 {
		s.Settings.NumberFormat = 0x202f
	}
	if s.Settings.Limits.Page == 0 {
		s.Settings.Limits.Page = 10
	}
	if s.Settings.Limits.Ref == 0 {
		s.Settings.Limits.Ref = 10
	}

	s.Code = strings.ToLower(s.Code)

	if s.CreatedAt.IsZero() {
		s.CreatedAt = Now()
	} else {
		t := Now()
		s.UpdatedAt = &t
	}
}

var noUnderscore = time.Date(2020, 03, 20, 0, 0, 0, 0, time.UTC)

// Validate the object.
func (s *Site) Validate(ctx context.Context) error {
	v := zvalidate.New()

	v.Required("code", s.Code)
	v.Required("state", s.State)
	v.Required("plan", s.Plan)
	v.Include("state", s.State, States)
	if s.Parent == nil {
		v.Include("plan", s.Plan, Plans)
	} else {
		v.Include("plan", s.Plan, []string{PlanChild})
	}

	v.Range("settings.limits.page", int64(s.Settings.Limits.Page), 1, 25)
	v.Range("settings.limits.ref", int64(s.Settings.Limits.Ref), 1, 25)

	if s.Settings.DataRetention > 0 {
		v.Range("settings.data_retention", int64(s.Settings.DataRetention), 14, 0)
	}

	if len(s.Settings.IgnoreIPs) > 0 {
		for _, ip := range s.Settings.IgnoreIPs {
			v.IP("settings.ignore_ips", ip)
		}
	}

	v.Domain("link_domain", s.LinkDomain)
	v.Len("code", s.Code, 2, 50)
	v.Exclude("code", s.Code, reserved)
	// TODO: compat with older requirements, otherwise various update functions
	// will error out.
	if !s.CreatedAt.IsZero() && s.CreatedAt.Before(noUnderscore) {
		for _, c := range s.Code {
			if !(c == '-' || c == '_' || (c >= '0' && c <= '9') || (c >= 'a' && c <= 'z')) {
				v.Append("code", fmt.Sprintf("%q not allowed; characters are limited to '_', '-', a to z, and numbers", c))
				break
			}
		}
		if len(s.Code) > 0 && (s.Code[0] == '_' || s.Code[0] == '-') { // Special domains, like _acme-challenge.
			v.Append("code", "cannot start with underscore or dash (_, -)")
		}
	} else {
		labels := v.Hostname("code", s.Code)
		if len(labels) > 1 {
			v.Append("code", "cannot contain '.'")
		}
	}

	if s.Cname != nil {
		v.Len("cname", *s.Cname, 4, 255)
		v.Domain("cname", *s.Cname)
		if cfg.Saas && strings.HasSuffix(*s.Cname, cfg.Domain) {
			v.Append("cname", "cannot end with %q", cfg.Domain)
		}

		var cname uint8
		err := zdb.MustGet(ctx).GetContext(ctx, &cname,
			`select 1 from sites where lower(cname)=lower($1) and id!=$2 limit 1`,
			s.Cname, s.ID)
		if err != nil && err != sql.ErrNoRows {
			return err
		}
		if cname == 1 {
			v.Append("cname", "already exists")
		}
	}

	if s.Stripe != nil && !strings.HasPrefix(*s.Stripe, "cus_") {
		v.Append("stripe", "not a valid Stripe customer ID")
	}

	if !v.HasErrors() {
		var code uint8
		err := zdb.MustGet(ctx).GetContext(ctx, &code,
			`select 1 from sites where lower(code)=lower($1) and id!=$2 limit 1`,
			s.Code, s.ID)
		if err != nil && err != sql.ErrNoRows {
			return err
		}
		if code == 1 {
			v.Append("code", "already exists")
		}
	}

	return v.ErrorOrNil()
}

// Insert a new row.
func (s *Site) Insert(ctx context.Context) error {
	if s.ID > 0 {
		return errors.New("ID > 0")
	}

	s.Defaults(ctx)
	err := s.Validate(ctx)
	if err != nil {
		return err
	}

	query := `insert into sites
		(parent, code, cname, link_domain, settings, plan, created_at)
		values ($1, $2, $3, $4, $5, $6, $7)`
	args := []interface{}{s.Parent, s.Code, s.Cname, s.LinkDomain, s.Settings,
		s.Plan, s.CreatedAt.Format(zdb.Date)}
	if cfg.PgSQL {
		err = zdb.MustGet(ctx).GetContext(ctx, &s.ID, query+" returning id", args...)
		if err != nil {
			if zdb.ErrUnique(err) {
				return guru.New(400, "this site already exists: code or domain must be unique")
			}
			return errors.Wrap(err, "Site.Insert")
		}
		return nil
	}

	res, err := zdb.MustGet(ctx).ExecContext(ctx, query, args...)
	if err != nil {
		if zdb.ErrUnique(err) {
			return guru.New(400, "this site already exists: code or domain must be unique")
		}
		return errors.Wrap(err, "Site.Insert")
	}
	s.ID, err = res.LastInsertId()
	return errors.Wrap(err, "Site.Insert")
}

// Update existing.
func (s *Site) Update(ctx context.Context) error {
	if s.ID == 0 {
		return errors.New("ID == 0")
	}

	s.Defaults(ctx)
	err := s.Validate(ctx)
	if err != nil {
		if err != nil {
			return err
		}
	}

	_, err = zdb.MustGet(ctx).ExecContext(ctx,
		`update sites set settings=$1, cname=$2, link_domain=$3, updated_at=$4 where id=$5`,
		s.Settings, s.Cname, s.LinkDomain, s.UpdatedAt.Format(zdb.Date), s.ID)
	return errors.Wrap(err, "Site.Update")
}

// UpdateStripe sets the Stripe customer ID.
func (s *Site) UpdateStripe(ctx context.Context, stripeID, plan string) error {
	if s.ID == 0 {
		return errors.New("ID == 0")
	}

	s.Defaults(ctx)
	err := s.Validate(ctx)
	if err != nil {
		return err
	}

	s.Stripe = &stripeID
	s.Plan = plan
	_, err = zdb.MustGet(ctx).ExecContext(ctx,
		`update sites set stripe=$1, plan=$2, updated_at=$3 where id=$4`,
		s.Stripe, s.Plan, s.UpdatedAt.Format(zdb.Date), s.ID)
	return errors.Wrap(err, "Site.UpdateStripe")
}

// Delete a site.
func (s *Site) Delete(ctx context.Context) error {
	if s.ID == 0 {
		return errors.New("ID == 0")
	}

	t := Now()
	_, err := zdb.MustGet(ctx).ExecContext(ctx,
		`update sites set state=$1, updated_at=$2 where id=$3 or parent=$3`,
		StateDeleted, t.Format(zdb.Date), s.ID)
	if err != nil {
		return errors.Wrap(err, "Site.Delete")
	}
	s.ID = 0
	s.UpdatedAt = &t
	s.State = StateDeleted
	return nil
}

// ByID gets a site by ID.
func (s *Site) ByID(ctx context.Context, id int64) error {
	return errors.Wrap(zdb.MustGet(ctx).GetContext(ctx, s,
		`select * from sites where id=$1 and state=$2`,
		id, StateActive), "Site.ByID")
}

// ByHost gets a site by host name.
func (s *Site) ByHost(ctx context.Context, host string) error {
	l := zlog.Module("Site.ByHost").Fields(zlog.F{
		"host":       host,
		"cfg.Domain": cfg.Domain,
	})

	// Custom domain or serve.
	if cfg.Serve || !strings.HasSuffix(host, cfg.Domain) {
		l.Debug("by cname")
		return errors.Wrap(zdb.MustGet(ctx).GetContext(ctx, s,
			`select * from sites where lower(cname)=lower($1) and state=$2`,
			zhttp.RemovePort(host), StateActive), "site.ByHost: from custom domain")
	}

	// Get from code (e.g. "arp242" in "arp242.goatcounter.com").
	p := strings.Index(host, ".")
	if p == -1 {
		return fmt.Errorf("Site.ByHost: no subdomain in host %q", host)
	}

	l.Debug("by code")
	return errors.Wrap(zdb.MustGet(ctx).GetContext(ctx, s,
		`select * from sites where lower(code)=lower($1) and state=$2`,
		host[:p], StateActive), "site.ByHost: from code")
}

// ListSubs lists all subsites, including the current site and parent.
func (s *Site) ListSubs(ctx context.Context) ([]string, error) {
	col := "code"
	if cfg.Serve {
		col = "cname"
	}
	var codes []string
	err := zdb.MustGet(ctx).SelectContext(ctx, &codes, `
		select `+col+` from sites
		where state=$1 and (parent=$2 or id=$2) or (
			parent = (select parent from sites where id=$2) or
			id     = (select parent from sites where id=$2)
		) and state=$1
		order by code
		`, StateActive, s.ID)
	return codes, errors.Wrap(err, "Site.ListSubs")
}

// Domain gets the global default domain, or this site's configured custom
// domain.
func (s Site) Domain() string {
	if s.Cname != nil {
		return *s.Cname
	}
	return cfg.Domain
}

// URL to this site.
func (s Site) URL() string {
	if s.Cname != nil {
		return fmt.Sprintf("http%s://%s%s",
			map[bool]string{true: "s", false: ""}[cfg.Prod],
			*s.Cname, cfg.Port)
	}

	return fmt.Sprintf("http%s://%s.%s%s",
		map[bool]string{true: "s", false: ""}[cfg.Prod],
		s.Code, cfg.Domain, cfg.Port)
}

// PlanCustomDomain reports if this site's plan allows custom domains.
func (s Site) PlanCustomDomain(ctx context.Context) bool {
	if s.Parent != nil {
		var ps Site
		err := ps.ByID(ctx, *s.Parent)
		if err != nil {
			zlog.Error(err)
			return false
		}
		return ps.PlanCustomDomain(ctx)
	}

	return s.Plan == PlanPersonalPlus || s.Plan == PlanBusiness || s.Plan == PlanBusinessPlus
}

// IDOrParent gets this site's ID or the parent ID if that's set.
func (s Site) IDOrParent() int64 {
	if s.Parent != nil {
		return *s.Parent
	}
	return s.ID
}

var trialPeriod = time.Hour * 24 * 14

func (s Site) ShowPayBanner(ctx context.Context) bool {
	if s.Parent != nil {
		var ps Site
		err := ps.ByID(ctx, *s.Parent)
		if err != nil {
			zlog.Error(err)
			return false
		}
		return ps.ShowPayBanner(ctx)
	}

	if s.Stripe != nil {
		return false
	}
	return -Now().Sub(s.CreatedAt.Add(trialPeriod)) < 0
}

func (s Site) FreePlan() bool {
	return s.Stripe != nil && strings.HasPrefix(*s.Stripe, "cus_free_")
}

func (s Site) PayExternal() string {
	if s.Stripe == nil {
		return ""
	}

	if strings.HasPrefix(*s.Stripe, "cus_github_") {
		return "GitHub Sponsors"
	}
	if strings.HasPrefix(*s.Stripe, "cus_patreon_") {
		return "Patreon"
	}
	return ""
}

func (s Site) DeleteOlderThan(ctx context.Context, days int) error {
	if days < 14 {
		return fmt.Errorf("days must be at least 14: %d", days)
	}

	return zdb.TX(ctx, func(ctx context.Context, tx zdb.DB) error {
		ival := interval(days)
		_, err := tx.ExecContext(ctx,
			`delete from hits where site=$1 and created_at < `+ival,
			s.ID)
		if err != nil {
			return errors.Wrap(err, "Site.DeleteOlderThan: delete sites")
		}

		_, err = tx.ExecContext(ctx,
			`delete from hit_counts where site=$1 and hour < `+ival,
			s.ID)
		if err != nil {
			return errors.Wrap(err, "Site.DeleteOlderThan: delete sites")
		}

		for _, t := range statTables {
			_, err := tx.ExecContext(ctx,
				`delete from `+t+` where site=$1 and day < `+ival,
				s.ID)
			if err != nil {
				return errors.Wrap(err, "Site.DeleteOlderThan: delete "+t)
			}
		}

		return nil
	})
}

// Admin reports if this site is an admin.
func (s Site) Admin() bool {
	return s.ID == 1
}

// Sites is a list of sites.
type Sites []Site

// List all sites.
func (s *Sites) List(ctx context.Context) error {
	return errors.Wrap(zdb.MustGet(ctx).SelectContext(ctx, s,
		`select * from sites where state=$1 order by created_at desc`,
		StateActive), "Sites.List")
}

// ListCnames all sites that have CNAME set.
func (s *Sites) ListCnames(ctx context.Context) error {
	return errors.Wrap(zdb.MustGet(ctx).SelectContext(ctx, s,
		`select * from sites where state=$1 and cname is not null order by created_at desc`,
		StateActive), "Sites.List")
}

// ListSubs lists all subsites for the current site.
func (s *Sites) ListSubs(ctx context.Context) error {
	return errors.Wrap(zdb.MustGet(ctx).SelectContext(ctx, s,
		`select * from sites where parent=$1 and state=$2 order by code`,
		MustGetSite(ctx).ID, StateActive), "Sites.ListSubs")
}

// ContainsCNAME reports if there is a site with this CNAME set.
func (s *Sites) ContainsCNAME(ctx context.Context, cname string) (bool, error) {
	var ok bool
	err := zdb.MustGet(ctx).GetContext(ctx, &ok,
		`select 1 from sites where lower(cname)=lower($1) limit 1`, cname)
	return ok, errors.Wrap(err, "Sites.ContainsCNAME")
}

// OldSoftDeleted finds all sites which have been soft-deleted more than a week
// ago.
func (s *Sites) OldSoftDeleted(ctx context.Context) error {
	return errors.Wrap(zdb.MustGet(ctx).SelectContext(ctx, s, fmt.Sprintf(
		`select * from sites where state=$1 and updated_at < %s`, interval(7)),
		StateDeleted), "Sites.OldSoftDeleted")
}

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

	"github.com/pkg/errors"
	"github.com/teamwork/guru"
	"zgo.at/goatcounter/cfg"
	"zgo.at/utils/jsonutil"
	"zgo.at/utils/sqlutil"
	"zgo.at/zdb"
	"zgo.at/zhttp"
	"zgo.at/zlog"
	"zgo.at/zvalidate"
)

// Plan column values.
const (
	PlanPersonal     = "personal"
	PlanBusiness     = "business"
	PlanBusinessPlus = "businessplus"
	PlanChild        = "child"
)

var Plans = []string{PlanPersonal, PlanBusiness, PlanBusinessPlus}

var reserved = []string{
	"www", "mail", "smtp", "imap", "static",
	"admin", "ns1", "ns2", "m", "mobile", "api",
	"dev", "test", "beta", "new", "staging", "debug", "pprof",
	"chat",
}

// Site is a single site which is sending newsletters (i.e. it's a "customer").
type Site struct {
	ID     int64  `db:"id"`
	Parent *int64 `db:"parent"`

	Name         string       `db:"name"`        // Any name for the website.
	Cname        *string      `db:"cname"`       // Custom domain, e.g. "stats.example.com"
	Code         string       `db:"code"`        // Domain code (arp242, which makes arp242.goatcounter.com)
	LinkDomain   string       `db:"link_domain"` // Site domain for linking (www.arp242.net).
	Plan         string       `db:"plan"`
	Stripe       *string      `db:"stripe"`
	Settings     SiteSettings `db:"settings"`
	LastStat     *time.Time   `db:"last_stat"`
	ReceivedData bool         `db:"received_data"`

	State     string     `db:"state"`
	CreatedAt time.Time  `db:"created_at"`
	UpdatedAt *time.Time `db:"updated_at"`
}

type SiteSettings struct {
	Public          bool               `json:"public"`
	TwentyFourHours bool               `json:"twenty_four_hours"`
	DateFormat      string             `json:"date_format"`
	NumberFormat    rune               `json:"number_format"`
	DataRetention   int                `json:"data_retention"`
	IgnoreIPs       sqlutil.StringList `json:"ignore_ips"`
	Limits          struct {
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
		s.CreatedAt = time.Now().UTC()
	} else {
		t := time.Now().UTC()
		s.UpdatedAt = &t
	}
}

// Validate the object.
func (s *Site) Validate(ctx context.Context) error {
	v := zvalidate.New()

	v.Required("name", s.Name)
	v.Required("code", s.Code)
	v.Required("state", s.State)
	v.Required("plan", s.Plan)
	v.Include("state", s.State, States)
	if s.Parent == nil {
		v.Include("plan", s.Plan, Plans)
	} else {
		v.Include("plan", s.Plan, []string{PlanChild})
	}

	if s.Settings.DataRetention > 0 {
		v.Range("settings.data_retention", int64(s.Settings.DataRetention), 14, 0)
	}

	if len(s.Settings.IgnoreIPs) > 0 {
		for _, ip := range s.Settings.IgnoreIPs {
			v.IP("settings.ignore_ips", ip)
		}
	}

	v.Domain("link_domain", s.LinkDomain)
	v.Len("code", s.Code, 1, 50)
	v.Len("name", s.Name, 4, 255)
	v.Exclude("code", s.Code, reserved)
	if s.Cname != nil {
		v.Len("cname", *s.Cname, 4, 255)
		v.Domain("cname", *s.Cname)
		if strings.HasSuffix(*s.Cname, cfg.Domain) {
			v.Append("cname", "cannot end with %q", cfg.Domain)
		}
	}

	if s.Stripe != nil && !strings.HasPrefix(*s.Stripe, "cus_") {
		v.Append("stripe", "not a valid Stripe customer ID")
	}

	for _, c := range s.Code {
		if !(c == '-' || c == '_' || (c >= '0' && c <= '9') || (c >= 'a' && c <= 'z')) {
			v.Append("code", fmt.Sprintf("%q not allowed; characters are limited to '_', '-', a to z, and numbers", c))
			break
		}
	}
	if len(s.Code) > 0 && (s.Code[0] == '_' || s.Code[0] == '-') { // Special domains, like _acme-challenge.
		v.Append("code", "cannot start with underscore or dash (_, -)")
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

	res, err := zdb.MustGet(ctx).ExecContext(ctx,
		`insert into sites (parent, code, name, settings, plan, created_at) values ($1, $2, $3, $4, $5, $6)`,
		s.Parent, s.Code, s.Name, s.Settings, s.Plan, s.CreatedAt.Format(zdb.Date))
	if err != nil {
		if zdb.UniqueErr(err) {
			return guru.New(400, "this site already exists: name and code must be unique")
		}
		return errors.Wrap(err, "Site.Insert")
	}

	if cfg.PgSQL {
		var ns Site
		err = ns.ByHost(ctx, s.Code+"."+cfg.Domain)
		s.ID = ns.ID
	} else {
		s.ID, err = res.LastInsertId()
	}
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
		return err
	}

	_, err = zdb.MustGet(ctx).ExecContext(ctx,
		`update sites set name=$1, settings=$2, cname=$3, link_domain=$4, updated_at=$5 where id=$6`,
		s.Name, s.Settings, s.Cname, s.LinkDomain, s.UpdatedAt.Format(zdb.Date), s.ID)
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

	_, err := zdb.MustGet(ctx).ExecContext(ctx,
		`update sites set state=$1 where id=$2`,
		StateDeleted, s.ID)
	if err != nil {
		return errors.Wrap(err, "Site.Delete")
	}
	s.ID = 0
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
	// Custom domain.
	if !strings.HasSuffix(host, cfg.Domain) {
		return errors.Wrap(zdb.MustGet(ctx).GetContext(ctx, s,
			`select * from sites where lower(cname)=lower($1) and state=$2`,
			zhttp.RemovePort(host), StateActive), "site.ByHost: from custom domain")
	}

	// Get from code (e.g. "arp242" in "arp242.goatcounter.com").
	p := strings.Index(host, ".")
	if p == -1 {
		return fmt.Errorf("Site.ByHost: no subdomain in host %q", host)
	}

	return errors.Wrap(zdb.MustGet(ctx).GetContext(ctx, s,
		`select * from sites where lower(code)=lower($1) and state=$2`,
		host[:p], StateActive), "site.ByHost: from code")
}

// ListSubs lists all subsites, including the current site and parent.
func (s *Site) ListSubs(ctx context.Context) ([]string, error) {
	var codes []string
	err := zdb.MustGet(ctx).SelectContext(ctx, &codes, `
		select code from sites
		where state=$2 and (parent=$1 or id=$1) or (
			parent = (select parent from sites where id=$1) or
			id     = (select parent from sites where id=$1)
		) and state=$2
		order by code
	`, s.ID, StateActive)
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
		return fmt.Sprintf("http%s://%s",
			map[bool]string{true: "s", false: ""}[cfg.Prod],
			*s.Cname)
	}

	return fmt.Sprintf("http%s://%s.%s",
		map[bool]string{true: "s", false: ""}[cfg.Prod],
		s.Code, cfg.Domain)
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

	return s.Plan == PlanBusiness || s.Plan == PlanBusinessPlus
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
	return -time.Now().UTC().Sub(s.CreatedAt.Add(trialPeriod)) < 0
}

func (s Site) FreePlan() bool {
	return s.Stripe != nil && strings.HasPrefix(*s.Stripe, "cus_free_")
}

func (s Site) DeleteOlderThan(ctx context.Context, days int) error {
	if days < 14 {
		return fmt.Errorf("days must be at least 14: %d", days)
	}

	return zdb.TX(ctx, func(ctx context.Context, tx zdb.DB) error {
		ival := interval(days)
		r, err := tx.ExecContext(ctx,
			`delete from hits where site=$1 and created_at < `+ival,
			s.ID)
		if err != nil {
			return errors.Wrap(err, "Site.DeleteOlderThan: delete sites")
		}

		n, _ := r.RowsAffected()
		zlog.Module("DeleteOlderThan").Fields(zlog.F{"site": s.ID, "hits": n}).Print("deleted hits")

		for _, t := range []string{"hit_stats", "browser_stats", "location_stats"} {
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

// Sites is a list of sites.
type Sites []Site

// List all sites.
func (u *Sites) List(ctx context.Context) error {
	return errors.Wrap(zdb.MustGet(ctx).SelectContext(ctx, u,
		`select * from sites where state=$1 order by created_at desc`,
		StateActive), "Sites.List")
}

// ListSubs lists all subsites for the current site.
func (u *Sites) ListSubs(ctx context.Context) error {
	return errors.Wrap(zdb.MustGet(ctx).SelectContext(ctx, u,
		`select * from sites where parent=$1 and state=$2 order by code`,
		MustGetSite(ctx).ID, StateActive), "Sites.ListSubs")
}

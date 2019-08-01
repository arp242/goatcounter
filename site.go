// Copyright © 2019 Martin Tournoij <martin@arp242.net>
// This file is part of GoatCounter and published under the terms of the AGPLv3,
// which can be found in the LICENSE file or at gnu.org/licenses/agpl.html

package goatcounter

import (
	"context"
	"database/sql/driver"
	"encoding/json"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/teamwork/guru"
	"github.com/teamwork/utils/jsonutil"
	"github.com/teamwork/validate"
)

// Plan column values.
const (
	PlanPersonal   = "p"
	PlanBusiness   = "b"
	PlanEnterprise = "e"
)

var Plans = []string{PlanPersonal, PlanBusiness, PlanEnterprise}

var reserved = []string{
	"goatcounter", "goatcounters",
	"www", "mail", "smtp", "imap", "static",
	"admin", "ns1", "ns2", "m", "mobile", "api",
}

// Site is a single site which is sending newsletters (i.e. it's a "customer").
type Site struct {
	ID int64 `db:"id"`

	Domain       string       `db:"domain"` // Domain for which the service is (arp242.net)
	Code         string       `db:"code"`   // Domain code (arp242, which makes arp242.goatcounter.com)
	Plan         string       `db:"plan"`
	Settings     SiteSettings `db:"settings"`
	LastStat     *time.Time   `db:"last_stat"`
	ReceivedData bool         `db:"received_data"`

	State     string     `db:"state"`
	CreatedAt time.Time  `db:"created_at"`
	UpdatedAt *time.Time `db:"updated_at"`
}

type SiteSettings struct {
	Public          bool   `json:"public"`
	DateFormat      string `json:"date_format"`
	TwentyFourHours bool   `json:"twenty_four_hours"`
	Limits          struct {
		Page    int `json:"page"`
		Ref     int `json:"ref"`
		Browser int `json:"browser"`
	} `json:"limits"`
}

func (ss SiteSettings) String() string { return string(jsonutil.MustMarshal(ss)) }

// Value implements the SQL Value function to determine what to store in the DB.
func (ss SiteSettings) Value() (driver.Value, error) { return json.Marshal(ss) }

// Scan converts the data returned from the DB into the struct.
func (ss *SiteSettings) Scan(v interface{}) error { return json.Unmarshal(v.([]byte), ss) }

// Defaults sets fields to default values, unless they're already set.
func (s *Site) Defaults(ctx context.Context) {
	if s.State == "" {
		s.State = StateActive
	}

	if s.Settings.DateFormat == "" {
		s.Settings.DateFormat = "2006-01-02"
	}

	if s.Settings.Limits.Page == 0 {
		s.Settings.Limits.Page = 20
	}
	if s.Settings.Limits.Ref == 0 {
		s.Settings.Limits.Ref = 10
	}
	if s.Settings.Limits.Browser == 0 {
		s.Settings.Limits.Browser = 20
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
	v := validate.New()

	v.Required("domain", s.Domain)
	v.Required("code", s.Code)
	v.Required("state", s.State)
	v.Required("plan", s.Plan)
	v.Include("state", s.State, States)
	v.Include("plan", s.Plan, Plans)

	v.Len("code", s.Code, 0, 50)
	v.Len("domain", s.Domain, 0, 255)
	v.Domain("domain", s.Domain)
	v.Exclude("domain", s.Domain, reserved)

	for _, c := range s.Code {
		if c == 95 || (c >= 48 && c <= 57) || (c >= 97 && c <= 122) {
			v.Append("code", "characters are limited to '_', a to z, and numbers")
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

	res, err := MustGetDB(ctx).ExecContext(ctx,
		`insert into sites (code, domain, settings, created_at) values ($1, $2, $3, $4)`,
		s.Code, s.Domain, s.Settings, sqlDate(s.CreatedAt))
	if err != nil {
		if uniqueErr(err) {
			return guru.New(400, "this site already exists")
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
		return err
	}

	_, err = MustGetDB(ctx).ExecContext(ctx,
		`update sites set domain=$1, settings=$2, updated_at=$3 where id=$4`,
		s.Domain, s.Settings, sqlDate(*s.UpdatedAt), s.ID)
	return errors.Wrap(err, "Site.Update")
}

// ByID gets a site by ID.
func (s *Site) ByID(ctx context.Context, id int64) error {
	db := MustGetDB(ctx)
	return errors.Wrap(db.GetContext(ctx, s, `select * from sites where id=$1 and state=$2`,
		id, StateActive), "Site.ByID")
}

// ByCode gets a site by subdomain code.
func (s *Site) ByCode(ctx context.Context, code string) error {
	db := MustGetDB(ctx)
	return errors.Wrap(db.GetContext(ctx, s, `select * from sites where code=$1 and state=$2`,
		code, StateActive), "Site.ByCode")
}

// Sites is a list of sites.
type Sites []Site

// List all sites.
func (u *Sites) List(ctx context.Context) error {
	db := MustGetDB(ctx)
	return errors.Wrap(db.SelectContext(ctx, u,
		`select * from sites order by created_at desc`),
		"Sites.List")
}

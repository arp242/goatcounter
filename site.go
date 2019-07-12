package goatcounter

import (
	"context"
	"database/sql/driver"
	"encoding/json"
	"time"

	"github.com/pkg/errors"
	"github.com/teamwork/guru"
	"github.com/teamwork/utils/jsonutil"
	"github.com/teamwork/validate"
)

var reserved = []string{
	"goatcounter", "goatcounters",
	"www", "mail", "smtp", "imap", "static",
	"admin", "ns1", "ns2", "m", "mobile", "api",
}

// Site is a single site which is sending newsletters (i.e. it's a "customer").
type Site struct {
	ID int64 `db:"id"`

	Domain   string       `db:"domain"` // Domain for which the service is (arp242.net)
	Code     string       `db:"code"`   // Domain code (arp242, which makes arp242.goatcounter.com)
	Settings SiteSettings `db:"settings"`

	State     string     `db:"state"`
	CreatedAt time.Time  `db:"created_at"`
	UpdatedAt *time.Time `db:"updated_at"`

	Name string `db:"name"` // TODO(v1): remove, just here for compat since we can't remove cols from SQLite
}

type SiteSettings struct {
	Public          bool   `json:"public"`
	TimeFormat      string `json:"time_format"`
	TwentyFourHours bool   `json:"twenty_four_hours"`
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

	if s.Settings.TimeFormat == "" {
		s.Settings.TimeFormat = "2006-01-02"
	}

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
	v.Required("state", s.State)
	v.Include("state", s.State, States)

	v.Len("domain", s.Domain, 0, 255)
	v.Domain("domain", s.Domain)
	v.Exclude("domain", s.Domain, reserved)

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
		`insert into sites (code, domain, settings) values ($1, $2, $3, $4)`,
		s.Code, s.Domain, s.Settings)
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
		s.Domain, s.Settings, s.UpdatedAt, s.ID)
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

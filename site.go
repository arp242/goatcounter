package goatcounter

import (
	"context"
	"time"

	"github.com/pkg/errors"
	"github.com/teamwork/guru"
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

	Domain string `db:"domain"` // Domain for which the service is (arp242.net)
	Name   string `db:"name"`   // Site name, for humans (arp242.net: Martin's website)
	Code   string `db:"code"`   // Domain code (arp242, which makes arp242.goatcounter.com)

	State     string     `db:"state"`
	CreatedAt time.Time  `db:"created_at"`
	UpdatedAt *time.Time `db:"updated_at"`
}

// Defaults sets fields to default values, unless they're already set.
func (s *Site) Defaults(ctx context.Context) {
	if s.State == "" {
		s.State = StateActive
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

	v.Required("name", s.Name)
	v.Required("domain", s.Domain)
	v.Required("state", s.State)
	v.Include("state", s.State, States)

	v.Len("name", s.Name, 0, 50)
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

	db := MustGetDB(ctx)
	s.Defaults(ctx)
	err := s.Validate(ctx)
	if err != nil {
		return err
	}

	res, err := db.ExecContext(ctx, `insert into sites (name, code, domain) values ($1, $2, $3)`,
		s.Name, s.Code, s.Domain)
	if err != nil {
		if uniqueErr(err) {
			return guru.New(400, "this site already exists")
		}
		return errors.Wrap(err, "Site.Insert")
	}

	s.ID, err = res.LastInsertId()
	return errors.Wrap(err, "Site.Insert")
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

package goatcounter

import (
	"context"
	"time"

	"github.com/pkg/errors"
	"github.com/teamwork/guru"
	"github.com/teamwork/validate"
	"zgo.at/goatcounter/cfg"
)

type Domain struct {
	ID   int64 `db:"id"`
	Site int64 `db:"site"`

	Domain       string  `db:"domain"`
	DisplayOrder int     `db:"display_order"`
	Color        *string `db:"color"`
	BgColor      *string `db:"bg_color"`

	State     string     `db:"state"`
	CreatedAt time.Time  `db:"created_at"`
	UpdatedAt *time.Time `db:"updated_at"`
}

var reserved = []string{
	"goatcounter", "goatcounters",
	"www", "mail", "smtp", "imap", "static",
	"admin", "ns1", "ns2", "m", "mobile", "api",
}

// Defaults sets fields to default values, unless they're already set.
func (d *Domain) Defaults(ctx context.Context) {
	if d.CreatedAt.IsZero() {
		d.CreatedAt = time.Now().UTC()
	} else {
		t := time.Now().UTC()
		d.UpdatedAt = &t
	}
}

// Validate the object.
func (d *Domain) Validate(ctx context.Context) error {
	v := validate.New()

	v.Len("domain", d.Domain, 4, 255)
	v.Domain("domain", d.Domain)
	v.Exclude("domain", d.Domain, reserved)

	return v.ErrorOrNil()
}

// Insert a new row.
func (d *Domain) Insert(ctx context.Context) error {
	if d.ID > 0 {
		return errors.New("ID > 0")
	}

	d.Defaults(ctx)
	err := d.Validate(ctx)
	if err != nil {
		return err
	}

	res, err := MustGetDB(ctx).ExecContext(ctx,
		`insert into domains (site, domain, created_at) values ($1, $2, $3)`,
		d.Site, d.Domain, sqlDate(d.CreatedAt))
	if err != nil {
		if uniqueErr(err) {
			return guru.New(400, "this domain already exists")
		}
		return errors.Wrap(err, "Domain.Insert")
	}

	// Get last ID.
	if cfg.PgSQL {
		// TODO!
		// err := MustGetDB(ctx).QueryRowContext(ctx, query+` returning id`,
		//    d.Site, d.Domain, sqlDate(d.CreatedAt)).Scan(&d.ID)
		// var ns Domain
		// err = ns.ByDomain(ctx, d.Domain)
		// d.ID = ns.ID
	} else {
		d.ID, err = res.LastInsertId()
	}
	return errors.Wrap(err, "Domain.Insert")
}

// Update a row.
func (d *Domain) Update(ctx context.Context) error {
	if d.ID == 0 {
		return errors.New("ID == 0")
	}

	d.Defaults(ctx)
	err := d.Validate(ctx)
	if err != nil {
		return err
	}

	_, err = MustGetDB(ctx).ExecContext(ctx,
		`update domains set domain=$1, display_order=$2, color=$3, bg_color=$4, updated_at=$5 where id=$6`,
		d.Domain, d.DisplayOrder, d.Color, d.BgColor, sqlDate(*d.UpdatedAt), d.ID)
	return errors.Wrap(err, "Domain.Update")
}

// ByNameOrFirst gets a domain name by the domain name for the current site,
// falling back to the first one if name is empty.
func (d *Domain) ByNameOrFirst(ctx context.Context, name string) error {
	siteID := MustGetSite(ctx).ID
	var query string
	var args []interface{}
	if name == "" {
		query = `select * from domains where site=$1 order by display_order asc, created_at desc limit 1`
		args = []interface{}{siteID}
	} else {
		query = `select * from domains where site=$1 and lower(domain)=lower($2) limit 1`
		args = []interface{}{siteID, name}
	}

	err := MustGetDB(ctx).GetContext(ctx, d, query, args...)
	return errors.Wrap(err, "Domain.ByNameOrFirst")
}

type Domains []Domain

// List all domains for a site.
func (d *Domains) List(ctx context.Context, site int64) error {
	return errors.Wrap(MustGetDB(ctx).SelectContext(ctx, d,
		`select * from domains where site=$1 order by display_order asc, created_at desc`, site),
		"Domains.List")
}

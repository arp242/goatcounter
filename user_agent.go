// Copyright © 2019 Martin Tournoij <martin@arp242.net>
// This file is part of GoatCounter and published under the terms of the EUPL
// v1.2, which can be found in the LICENSE file or at http://eupl12.zgo.at

package goatcounter

import (
	"context"

	"zgo.at/errors"
	"zgo.at/gadget"
	"zgo.at/goatcounter/cfg"
	"zgo.at/zdb"
	"zgo.at/zvalidate"
)

type UserAgent struct {
	ID        int64  `db:"user_agent_id"`
	UserAgent string `db:"ua"`

	Bot            uint8  `db:"bot"`
	Browser        string `db:"browser"`
	BrowserVersion string `db:"browser_version"`
	System         string `db:"system"`
	SystemVersion  string `db:"system_version"`
}

func (p *UserAgent) Defaults(ctx context.Context) {
	ua := gadget.Parse(p.UserAgent)
	p.Browser, p.BrowserVersion = ua.BrowserName, ua.BrowserVersion
	p.System, p.SystemVersion = ua.OSName, ua.OSVersion
}

func (p *UserAgent) Validate(ctx context.Context) error {
	v := zvalidate.New()

	v.UTF8("browser", p.UserAgent)
	v.Len("browser", p.Browser, 0, 512)

	// v.Required("browser", p.Browser)
	// v.Required("browser_version", p.BrowserVersion)
	// v.Required("system", p.System)
	// v.Required("system_version", p.SystemVersion)

	return v.ErrorOrNil()
}

func (p *UserAgent) GetOrInsert(ctx context.Context) error {
	db := zdb.MustGet(ctx)

	p.Defaults(ctx)
	err := p.Validate(ctx)
	if err != nil {
		return err
	}

	row := db.QueryRowxContext(ctx, `/* Path.GetOrInsert */
		select * from user_agents
		where ua = $1
		limit 1`,
		p.UserAgent)
	if row.Err() != nil {
		return errors.Errorf("UserAgent.GetOrInsert select: %w", row.Err())
	}

	err = row.StructScan(p)
	if err != nil && !zdb.ErrNoRows(err) {
		return errors.Errorf("UserAgent.GetOrInsert select: %w", err)
	}
	if err == nil {
		return nil
	}

	// Insert new row.
	// TODO: shorten
	// TODO: isbot := isbot.Bot(r)

	query := `insert into user_agents
		(ua, bot, browser, browser_version, system, system_version) values
		($1, $2, $3, $4, $5, $6)`
	args := []interface{}{p.UserAgent, 0, p.Browser, p.BrowserVersion, p.System, p.SystemVersion}

	if cfg.PgSQL {
		err = db.GetContext(ctx, &p.ID, query+" returning id", args...)
		return errors.Wrap(err, "UserAgent.GetOrInsert insert")
	}

	r, err := db.ExecContext(ctx, query, args...)
	if err != nil {
		return errors.Errorf("UserAgent.GetOrInsert insert: %w", err)
	}
	p.ID, err = r.LastInsertId()
	if err != nil {
		return errors.Errorf("UserAgent.GetOrInsert insert: %w", err)
	}

	return nil
}

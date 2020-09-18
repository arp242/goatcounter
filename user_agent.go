// Copyright © 2019 Martin Tournoij <martin@arp242.net>
// This file is part of GoatCounter and published under the terms of the EUPL
// v1.2, which can be found in the LICENSE file or at http://eupl12.zgo.at

package goatcounter

import (
	"context"
	"fmt"
	"time"

	"zgo.at/errors"
	"zgo.at/gadget"
	"zgo.at/isbot"
	"zgo.at/zcache"
	"zgo.at/zdb"
	"zgo.at/zvalidate"
)

type UserAgent struct {
	ID        int64  `db:"user_agent_id"`
	UserAgent string `db:"ua"`

	Bot       uint8 `db:"bot"`
	BrowserID int64 `db:"browser_id"`
	SystemID  int64 `db:"system_id"`
}

func (p *UserAgent) Defaults(ctx context.Context) {
}

func (p *UserAgent) Validate(ctx context.Context) error {
	v := zvalidate.New()
	// UserAgent may be an empty string, as some browsers send that.
	v.UTF8("user_agent", p.UserAgent)
	return v.ErrorOrNil()
}

func (p *UserAgent) ByID(ctx context.Context, id int64) error {
	err := zdb.MustGet(ctx).GetContext(ctx, p, `/* UserAgent.ByID */
		select * from user_agents where user_agent_id=$1`, id)
	return errors.Wrapf(err, "UserAgent.ByID %d", id)
}

var cacheUA = zcache.New(1*time.Hour, 5*time.Minute)

func (p *UserAgent) GetOrInsert(ctx context.Context) error {
	shortUA := gadget.Shorten(p.UserAgent)

	c, ok := cacheUA.Get(shortUA)
	fmt.Println("CACHE", ok, c)
	if ok {
		*p = c.(UserAgent)
		cacheUA.Touch(shortUA, zcache.DefaultExpiration)
		return nil
	}

	p.Defaults(ctx)
	err := p.Validate(ctx)
	if err != nil {
		return err
	}

	row := zdb.MustGet(ctx).QueryRowxContext(ctx, `/* UserAgent.GetOrInsert */
		select * from user_agents where ua=$1 limit 1`, shortUA)
	if row.Err() != nil {
		return errors.Errorf("UserAgent.GetOrInsert select: %w", row.Err())
	}
	err = row.StructScan(p)
	if err != nil && !zdb.ErrNoRows(err) {
		return errors.Errorf("UserAgent.GetOrInsert select: %w", err)
	}
	if err == nil {
		cacheUA.SetDefault(shortUA, *p)
		return nil // Got a row already, no need for a new one.
	}

	var (
		ua      = gadget.Parse(p.UserAgent)
		browser Browser
		system  System
	)

	err = browser.GetOrInsert(ctx, ua.BrowserName, ua.BrowserVersion)
	if err != nil {
		return errors.Wrap(err, "UserAgent.GetOrInsert")
	}
	p.BrowserID = browser.ID

	err = system.GetOrInsert(ctx, ua.OSName, ua.OSVersion)
	if err != nil {
		return errors.Wrap(err, "UserAgent.GetOrInsert")
	}
	p.SystemID = system.ID

	// Insert new row.
	p.Bot = isbot.UserAgent(p.UserAgent)
	p.ID, err = insertWithID(ctx, "user_agent_id", `insert into user_agents
		(ua, bot, browser_id, system_id) values ($1, $2, $3, $4)`,
		shortUA, p.Bot, p.BrowserID, p.SystemID)
	if err != nil {
		return errors.Wrap(err, "UserAgent.GetOrInsert insert")
	}

	cacheUA.SetDefault(shortUA, *p)
	return nil
}

type Browser struct {
	ID      int64  `db:"browser_id"`
	Name    string `db:"name"`
	Version string `db:"version"`
}

func (b *Browser) GetOrInsert(ctx context.Context, name, version string) error {
	b.Name = name
	b.Version = version

	err := zdb.MustGet(ctx).GetContext(ctx, &b.ID,
		`select browser_id from browsers where name=$1 and version=$2`,
		name, version)
	if zdb.ErrNoRows(err) {
		b.ID, err = insertWithID(ctx, "browser_id",
			`insert into browsers (name, version) values ($1, $2)`,
			name, version)
	}
	return errors.Wrap(err, "Browser.GetOrInsert")
}

type System struct {
	ID      int64  `db:"system_id"`
	Name    string `db:"name"`
	Version string `db:"version"`
}

func (s *System) GetOrInsert(ctx context.Context, name, version string) error {
	s.Name = name
	s.Version = version

	err := zdb.MustGet(ctx).GetContext(ctx, &s.ID,
		`select system_id from systems where name=$1 and version=$2`,
		name, version)
	if zdb.ErrNoRows(err) {
		s.ID, err = insertWithID(ctx, "system_id",
			`insert into systems (name, version) values ($1, $2)`,
			name, version)
	}
	return errors.Wrap(err, "System.GetOrInsert")
}

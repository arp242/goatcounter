// Copyright © 2019 Martin Tournoij <martin@arp242.net>
// This file is part of GoatCounter and published under the terms of the EUPL
// v1.2, which can be found in the LICENSE file or at http://eupl12.zgo.at

package goatcounter

import (
	"context"
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

	Isbot     uint8 `db:"isbot"`
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
	err := zdb.Get(ctx, p, `/* UserAgent.ByID */
		select * from user_agents where user_agent_id=$1`, id)
	return errors.Wrapf(err, "UserAgent.ByID %d", id)
}

var (
	cacheUA       = zcache.New(1*time.Hour, 5*time.Minute)
	cacheBrowsers = zcache.New(1*time.Hour, 5*time.Minute)
	cacheSystems  = zcache.New(1*time.Hour, 5*time.Minute)
)

func (p *UserAgent) Update(ctx context.Context) error {
	if p.ID == 0 {
		panic("ID is 0")
	}

	var (
		changed = false
		ua      = gadget.Parse(p.UserAgent)
		browser Browser
		system  System
	)
	err := browser.GetOrInsert(ctx, ua.BrowserName, ua.BrowserVersion)
	if err != nil {
		return errors.Wrap(err, "UserAgent.Update")
	}
	if p.BrowserID != browser.ID {
		changed = true
		p.BrowserID = browser.ID
	}

	err = system.GetOrInsert(ctx, ua.OSName, ua.OSVersion)
	if err != nil {
		return errors.Wrap(err, "UserAgent.Update")
	}
	if p.SystemID != system.ID {
		changed = true
		p.SystemID = system.ID
	}

	bot := isbot.UserAgent(p.UserAgent)
	if bot != p.Isbot {
		changed = true
		p.Isbot = bot
	}

	if !changed {
		return nil
	}

	err = zdb.Exec(ctx,
		`update user_agents set isbot=$1, browser_id=$2, system_id=$3 where user_agent_id=$4`,
		p.Isbot, p.BrowserID, p.SystemID, p.ID)
	if err != nil {
		return errors.Wrap(err, "UserAgent.Update")
	}

	cacheUA.Delete(gadget.Shorten(p.UserAgent))
	return nil
}

func (p *UserAgent) GetOrInsert(ctx context.Context) error {
	shortUA := gadget.Shorten(p.UserAgent)

	c, ok := cacheUA.Get(shortUA)
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

	err = zdb.Get(ctx, p, `/* UserAgent.GetOrInsert */
		select * from user_agents where ua = $1 limit 1`, shortUA)
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
	p.Isbot = isbot.UserAgent(p.UserAgent)
	p.ID, err = zdb.InsertID(ctx, "user_agent_id",
		`insert into user_agents (ua, isbot, browser_id, system_id) values (?, ?, ?, ?)`,
		shortUA, p.Isbot, p.BrowserID, p.SystemID)
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
	k := name + version
	c, ok := cacheBrowsers.Get(k)
	if ok {
		*b = c.(Browser)
		cacheBrowsers.Touch(k, zcache.DefaultExpiration)
		return nil
	}

	b.Name = name
	b.Version = version

	err := zdb.Get(ctx, &b.ID,
		`select browser_id from browsers where name=$1 and version=$2`,
		name, version)
	if zdb.ErrNoRows(err) {
		b.ID, err = zdb.InsertID(ctx, "browser_id",
			`insert into browsers (name, version) values ($1, $2)`,
			name, version)
	}
	if err != nil {
		return errors.Wrapf(err, "Browser.GetOrInsert %q %q", name, version)
	}
	cacheBrowsers.SetDefault(k, *b)
	return nil
}

type System struct {
	ID      int64  `db:"system_id"`
	Name    string `db:"name"`
	Version string `db:"version"`
}

func (s *System) GetOrInsert(ctx context.Context, name, version string) error {
	k := name + version
	c, ok := cacheSystems.Get(k)
	if ok {
		*s = c.(System)
		cacheSystems.Touch(k, zcache.DefaultExpiration)
		return nil
	}

	s.Name = name
	s.Version = version

	err := zdb.Get(ctx, &s.ID,
		`select system_id from systems where name=$1 and version=$2`,
		name, version)
	if zdb.ErrNoRows(err) {
		s.ID, err = zdb.InsertID(ctx, "system_id",
			`insert into systems (name, version) values ($1, $2)`,
			name, version)
	}
	if err != nil {
		return errors.Wrapf(err, "System.GetOrInsert %q %q", name, version)
	}
	cacheSystems.SetDefault(k, *s)
	return nil
}

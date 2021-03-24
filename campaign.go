// Copyright © 2019 Martin Tournoij – This file is part of GoatCounter and
// published under the terms of a slightly modified EUPL v1.2 license, which can
// be found in the LICENSE file or at https://license.goatcounter.com

package goatcounter

import (
	"context"
	"time"

	"zgo.at/errors"
	"zgo.at/zdb"
	"zgo.at/zstd/ztime"
	"zgo.at/zvalidate"
)

// The value of one conversion.
//
// 50
// 50%
// 50$
// 50€
const (
	CampaignValueNumber = iota
	CampaignValuePercent
	CampaignValueMoney
)

// Total campaign goals
const (
	CampaignGoalConverstions = iota // 2% conversion
	CampaignGoalPercent             // 123 converstions
	CampaignGoalValue               // *n* value
)

type Campaign struct {
	ID     int64 `db:"campaign_id" json:"campaign_id"`
	SiteID int64 `db:"site_id" json:"site_id"`
	PathID int64 `db:"path_id" json:"path_id"` // Target path

	Name   string  `db:"name" json:"name"`
	Notes  string  `db:"notes" json:"notes"`
	Params Strings `db:"params" json:"params"` // as: "ref=foo", or multiple as "ref=foo,utm_x=bar"

	Value     *int    `db:"value" json:"value"` // Any number
	ValueUnit *string `db:"value_unit" json:"value_unit"`
	ValueDesc *string `db:"value_desc" json:"value_desc"` // "/month", "total", or just blank, so we can show "$4/month".
	Goal      *int    `db:"goal" json:"goal"`             // %, $, or number, depending on GoalType
	GoalType  *string `db:"goal_type" json:"goal_type"`   // "perc", "value", "number"

	State     string     `db:"state" json:"state"`
	CreatedAt time.Time  `db:"created_at" json:"created_at"`
	UpdatedAt *time.Time `db:"updated_at" json:"updated_at"`

	Path Path `db:"-" json:"-"`
}

func findCampaign(ctx context.Context) int64 {
	var cc Campaigns
	err := cc.List(ctx)
	_ = err
	for _, c := range cc {
		_ = c
	}
	for _, c := range MustGetSite(ctx).Settings.Campaigns {
		_ = c
	}
	return 1
}

func (c *Campaign) Defaults(ctx context.Context) {
	c.SiteID = MustGetSite(ctx).ID

	if c.State == "" {
		c.State = StateActive
	}

	if c.CreatedAt.IsZero() {
		c.CreatedAt = ztime.Now()
	}
}

func (c *Campaign) Validate() error {
	v := zvalidate.New()
	v.Required("name", c.Name)

	v.Required("state", c.State)
	v.Include("state", c.State, []string{StateActive, StateArchived, StateDeleted})

	return v.ErrorOrNil()
}

func (c *Campaign) Insert(ctx context.Context) error {
	if c.ID > 0 {
		return errors.Errorf("Campaign.Insert: c.ID>0: %d", c.ID)
	}

	c.Defaults(ctx)
	err := c.Validate()
	if err != nil {
		return errors.Wrap(err, "Campaign.Insert")
	}

	err = zdb.Exec(ctx, `insert into campaigns
		(site_id, path_id, name, notes, params, value, value_unit, value_desc, goal, goal_type, created_at) values (?)`,
		zdb.L{c.SiteID, c.PathID, c.Name, c.Notes, c.Params, c.Value, c.ValueUnit, c.ValueDesc, c.Goal, c.GoalType, ztime.Now()})
	return errors.Wrap(err, "Campaign.Insert")
}

func (c *Campaign) Update(ctx context.Context) error {
	if c.ID == 0 {
		return errors.New("Campaign.Insert: c.ID == 0")
	}

	c.Defaults(ctx)
	err := c.Validate()
	if err != nil {
		return errors.Wrap(err, "Campaign.Update")
	}

	err = zdb.Exec(ctx, `update campaigns set
		path_id=?, name=?, notes=?, params=?, value=?, value_unit=?, value_desc=?, goal=?, goal_type=?, updated_at=?
		where site_id=? and campaign_id=?`,
		c.PathID, c.Name, c.Notes, c.Params, c.Value, c.ValueUnit, c.ValueDesc, c.Goal, c.GoalType, ztime.Now(),
		c.SiteID, c.ID)
	return errors.Wrap(err, "Campaign.Update")
}

func (c *Campaign) SetState(ctx context.Context, state string) error {
	c.State = state
	err := c.Validate()
	if err != nil {
		return errors.Wrap(err, "Campaign.Archive")
	}

	err = zdb.Exec(ctx, `update campaigns set state=? where site_id=? and campaign_id=?`,
		state, MustGetSite(ctx).ID, c.ID)
	return errors.Wrap(err, "Campaign.Archive")
}

func (c *Campaign) ByID(ctx context.Context, id int64) error {
	err := zdb.Get(ctx, c, `select * from campaigns where campaign_id=? and site_id=?`,
		id, MustGetSite(ctx).ID)
	return errors.Wrap(err, "Campaign.ByID")
}

type Campaigns []Campaign

func (c *Campaigns) List(ctx context.Context) error {
	err := zdb.Select(ctx, c, `select * from campaigns where site_id=? and state=?`,
		MustGetSite(ctx).ID, StateActive)
	return errors.Wrap(err, "Campaigns.List")
}

func (c *Campaigns) ListArchived(ctx context.Context) error {
	err := zdb.Select(ctx, c, `select * from campaigns where site_id=? and state=?`,
		MustGetSite(ctx).ID, StateArchived)
	return errors.Wrap(err, "Campaigns.List")
}

// WithPaths adds the associated paths to these campaigns.
func (c *Campaigns) WithPaths(ctx context.Context) error {
	cc := *c
	if len(cc) == 0 {
		return nil
	}

	var (
		ids = make([]int64, 0, len(cc))    // List of IDs
		m   = make(map[int64]int, len(cc)) // Map path_id → index in this c
	)
	for i, camp := range cc {
		ids = append(ids, camp.PathID)
		m[camp.PathID] = i
	}

	paths := make(Paths, 0, len(cc))
	err := zdb.Select(ctx, &paths, `select * from paths where site_id=? and path_id in (?)`,
		MustGetSite(ctx).ID, ids)
	if err != nil {
		return errors.Wrap(err, "Campaigns.WithPaths")
	}

	for _, p := range paths {
		cc[m[p.ID]].Path = p
	}
	return nil
}

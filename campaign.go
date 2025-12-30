package goatcounter

import (
	"context"

	"zgo.at/errors"
	"zgo.at/zdb"
	"zgo.at/zvalidate"
)

type CampaignID int32

type Campaign struct {
	ID     CampaignID `db:"campaign_id,id" json:"campaign_id"`
	SiteID SiteID     `db:"site_id" json:"site_id"`
	Name   string     `db:"name" json:"name"`
}

func (Campaign) Table() string { return "campaigns" }

var _ zdb.Defaulter = &Campaign{}

func (c *Campaign) Defaults(ctx context.Context) {
	if c.SiteID == 0 {
		c.SiteID = MustGetSite(ctx).ID
	}
}

var _ zdb.Validator = &Campaign{}

func (c *Campaign) Validate(ctx context.Context) error {
	v := zvalidate.New()
	v.Required("name", c.Name)
	v.Required("site_id", c.SiteID)
	return v.ErrorOrNil()
}

func (c *Campaign) Insert(ctx context.Context) error {
	err := zdb.Insert(ctx, c)
	return errors.Wrap(err, "Campaign.Insert")
}

func (c *Campaign) ByName(ctx context.Context, name string) error {
	k := c.Name
	if cc, ok := cacheCampaigns(ctx).Get(k); ok {
		*c = *cc
		return nil
	}

	err := zdb.Get(ctx, c, `select * from campaigns where site_id=? and lower(name)=lower(?)`,
		MustGetSite(ctx).ID, name)
	if err != nil {
		return errors.Wrap(err, "Campaign.ByName")
	}

	cacheCampaigns(ctx).Set(k, c)
	return nil
}

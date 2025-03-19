package goatcounter

import (
	"context"

	"zgo.at/errors"
	"zgo.at/zdb"
	"zgo.at/zvalidate"
)

type Campaign struct {
	ID     int64  `db:"campaign_id" json:"campaign_id"`
	SiteID int64  `db:"site_id" json:"site_id"`
	Name   string `db:"name" json:"name"`
}

func (c *Campaign) Defaults(ctx context.Context) {}

func (c *Campaign) Validate() error {
	v := zvalidate.New()
	v.Required("name", c.Name)
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

	c.ID, err = zdb.InsertID(ctx, "campaign_id",
		`insert into campaigns (site_id, name) values (?, ?)`, MustGetSite(ctx).ID, c.Name)
	if err != nil {
		return errors.Wrap(err, "Campaign.Insert")
	}
	return nil
}

func (c *Campaign) ByName(ctx context.Context, name string) error {
	k := c.Name
	if cc, ok := cacheCampaigns(ctx).Get(k); ok {
		*c = *cc.(*Campaign)
		return nil
	}

	err := zdb.Get(ctx, c, `select * from campaigns where site_id=? and lower(name)=lower(?)`,
		MustGetSite(ctx).ID, name)
	if err != nil {
		return errors.Wrap(err, "Campaign.ByName")
	}

	cacheCampaigns(ctx).SetDefault(k, c)
	return nil
}

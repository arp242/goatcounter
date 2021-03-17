// Copyright © 2019 Martin Tournoij – This file is part of GoatCounter and
// published under the terms of a slightly modified EUPL v1.2 license, which can
// be found in the LICENSE file or at https://license.goatcounter.com

package goatcounter

import (
	"context"
	"database/sql/driver"
	"fmt"
	"time"

	"zgo.at/errors"
	"zgo.at/json"
	"zgo.at/zdb"
	"zgo.at/zstd/zcrypto"
	"zgo.at/zstd/zjson"
	"zgo.at/zvalidate"
)

type APIToken struct {
	ID     int64 `db:"api_token_id" json:"-"`
	SiteID int64 `db:"site_id" json:"-"`
	UserID int64 `db:"user_id" json:"-"`

	Name        string              `db:"name" json:"name"`
	Token       string              `db:"token" json:"-"`
	Permissions APITokenPermissions `db:"permissions" json:"permissions"`

	CreatedAt time.Time `db:"created_at" json:"-"`
}

// TODO: this shoud really be a bitmask; this is awkward to deal with.
type APITokenPermissions struct {
	Count      bool `db:"count" json:"count"`
	Export     bool `db:"export" json:"export"`
	SiteRead   bool `db:"site_read" json:"site_read"`
	SiteCreate bool `db:"site_create" json:"site_create"`
	SiteUpdate bool `db:"site_update" json:"site_update"`
}

func (tp APITokenPermissions) String() string { return string(zjson.MustMarshal(tp)) }

// Value implements the SQL Value function to determine what to store in the DB.
func (tp APITokenPermissions) Value() (driver.Value, error) { return json.Marshal(tp) }

// Scan converts the data returned from the DB into the struct.
func (tp *APITokenPermissions) Scan(v interface{}) error {
	switch vv := v.(type) {
	case []byte:
		return json.Unmarshal(vv, tp)
	case string:
		return json.Unmarshal([]byte(vv), tp)
	default:
		panic(fmt.Sprintf("unsupported type: %T", v))
	}
}

// Defaults sets fields to default values, unless they're already set.
func (t *APIToken) Defaults(ctx context.Context) {
	t.SiteID = MustGetSite(ctx).ID
	t.Token = zcrypto.Secret256()
	t.CreatedAt = Now()
}

func (t *APIToken) Validate(ctx context.Context) error {
	v := zvalidate.New()
	v.Required("name", t.Name)
	v.Required("site_id", t.SiteID)
	v.Required("user_id", t.SiteID)
	v.Required("token", t.Token)
	return v.ErrorOrNil()
}

// Insert a new row.
func (t *APIToken) Insert(ctx context.Context) error {
	if t.ID > 0 {
		return errors.New("ID > 0")
	}

	t.Defaults(ctx)
	err := t.Validate(ctx)
	if err != nil {
		return err
	}

	t.ID, err = zdb.InsertID(ctx, "api_token_id",
		`insert into api_tokens (site_id, user_id, name, token, permissions, created_at) values (?, ?, ?, ?, ?, ?)`,
		t.SiteID, GetUser(ctx).ID, t.Name, t.Token, t.Permissions, t.CreatedAt)
	return errors.Wrap(err, "APIToken.Insert")
}

func (t *APIToken) ByID(ctx context.Context, id int64) error {
	return errors.Wrapf(zdb.Get(ctx, t, `/* APIToken.ByID */
		select * from api_tokens where api_token_id=$1 and site_id=$2`,
		id, MustGetSite(ctx).ID), "APIToken.ByID %d", id)
}

func (t *APIToken) ByToken(ctx context.Context, token string) error {
	return errors.Wrap(zdb.Get(ctx, t,
		`/* APIToken.ByID */ select * from api_tokens where token=$1 and site_id=$2`,
		token, MustGetSite(ctx).ID), "APIToken.ByToken")
}

func (t *APIToken) Delete(ctx context.Context) error {
	err := zdb.Exec(ctx,
		`/* APIToken.Delete */ delete from api_tokens where api_token_id=$1 and site_id=$2`,
		t.ID, MustGetSite(ctx).ID)
	return errors.Wrapf(err, "APIToken.Delete %d", t.ID)
}

type APITokens []APIToken

func (t *APITokens) List(ctx context.Context) error {
	return errors.Wrap(zdb.Select(ctx, t,
		`select * from api_tokens where site_id=$1 and user_id=$2`,
		MustGetSite(ctx).ID, GetUser(ctx).ID), "APITokens.List")
}

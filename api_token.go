// Copyright © 2019 Martin Tournoij – This file is part of GoatCounter and
// published under the terms of a slightly modified EUPL v1.2 license, which can
// be found in the LICENSE file or at https://license.goatcounter.com

package goatcounter

import (
	"context"
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"

	"zgo.at/errors"
	"zgo.at/goatcounter/cfg"
	"zgo.at/zdb"
	"zgo.at/zhttp"
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

type APITokenPermissions struct {
	Count  bool `db:"count" json:"count"`
	Export bool `db:"export" json:"export"`
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
	t.Token = zhttp.Secret256()
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

	query := `insert into api_tokens
		(site_id, user_id, name, token, permissions, created_at)
		values ($1, $2, $3, $4, $5, $6)`
	args := []interface{}{t.SiteID, GetUser(ctx).ID, t.Name, t.Token, t.Permissions, t.CreatedAt.Format(zdb.Date)}

	if cfg.PgSQL {
		err := zdb.MustGet(ctx).GetContext(ctx, &t.ID, query+` returning api_token_id`, args...)
		return errors.Wrap(err, "APIToken.Insert")
	}

	res, err := zdb.MustGet(ctx).ExecContext(ctx, query, args...)
	if err != nil {
		return errors.Wrap(err, "APIToken.Insert")
	}
	t.ID, err = res.LastInsertId()
	return errors.Wrap(err, "APIToken.Insert")
}

func (t *APIToken) ByID(ctx context.Context, id int64) error {
	return errors.Wrapf(zdb.MustGet(ctx).GetContext(ctx, t,
		`/* APIToken.ByID */ select * from api_tokens where api_token_id=$1 and site_id=$2`,
		id, MustGetSite(ctx).ID), "APIToken.ByID %d", id)
}

func (t *APIToken) ByToken(ctx context.Context, token string) error {
	return errors.Wrap(zdb.MustGet(ctx).GetContext(ctx, t,
		`/* APIToken.ByID */ select * from api_tokens where token=$1 and site_id=$2`,
		token, MustGetSite(ctx).ID), "APIToken.ByToken")
}

func (t *APIToken) Delete(ctx context.Context) error {
	_, err := zdb.MustGet(ctx).ExecContext(ctx,
		`/* APIToken.Delete */ delete from api_tokens where api_token_id=$1 and site_id=$2`,
		t.ID, MustGetSite(ctx).ID)
	return errors.Wrapf(err, "APIToken.Delete %d", t.ID)
}

type APITokens []APIToken

func (t *APITokens) List(ctx context.Context) error {
	return errors.Wrap(zdb.MustGet(ctx).SelectContext(ctx, t,
		`select * from api_tokens where site_id=$1 and user_id=$2`,
		MustGetSite(ctx).ID, GetUser(ctx).ID), "APITokens.List")
}

// Copyright © Martin Tournoij – This file is part of GoatCounter and published
// under the terms of a slightly modified EUPL v1.2 license, which can be found
// in the LICENSE file or at https://license.goatcounter.com

package goatcounter

import (
	"context"
	"strings"
	"time"

	"zgo.at/errors"
	"zgo.at/zdb"
	"zgo.at/zstd/zcrypto"
	"zgo.at/zstd/zint"
	"zgo.at/zstd/ztime"
	"zgo.at/zstd/ztype"
)

// APIToken permissions.
//
// DO NOT change the values of these constants; they're stored in the database.
const (
	APIPermNothing    zint.Bitflag64 = 1 << iota
	APIPermCount                     // 2
	APIPermExport                    // 4
	APIPermSiteRead                  // 8
	APIPermSiteCreate                // 16
	APIPermSiteUpdate                // 32
	APIPermStats                     // 64
)

type APIToken struct {
	ID     int64 `db:"api_token_id" json:"-"`
	SiteID int64 `db:"site_id" json:"-"`
	UserID int64 `db:"user_id" json:"-"`

	Name        string         `db:"name" json:"name"`
	Token       string         `db:"token" json:"-"`
	Permissions zint.Bitflag64 `db:"permissions" json:"permissions"`

	CreatedAt  time.Time  `db:"created_at" json:"-"`
	LastUsedAt *time.Time `db:"last_used_at" json:"-"`
}

type PermissionFlag struct {
	Label, Help string
	Flag        zint.Bitflag64
}

// PermissionFlags returns a list of all flags we know for the Permissions settings.
func (t APIToken) PermissionFlags(only ...zint.Bitflag64) []PermissionFlag {
	if len(only) > 1 {
		for _, o := range only[1:] {
			only[0] |= o
		}
	}

	all := []PermissionFlag{
		{
			Label: "Record pageviews",
			Help:  "Record pageviews with /api/v0/count",
			Flag:  APIPermCount,
		},
		{
			Label: "Read statistics",
			Help:  "Get statistics out of GoatCounter",
			Flag:  APIPermStats,
		},
		{
			Label: "Export",
			Help:  "Export data with /api/v0/export",
			Flag:  APIPermExport,
		},
		{
			Label: "Read sites",
			Flag:  APIPermSiteRead,
		},
		{
			Label: "Create sites",
			Flag:  APIPermSiteCreate,
		},
		{
			Label: "Update sites",
			Flag:  APIPermSiteUpdate,
		},
	}

	if len(only) == 0 {
		return all
	}

	filter := make([]PermissionFlag, 0, len(all))
	for _, a := range all {
		if !only[0].Has(a.Flag) {
			continue
		}
		filter = append(filter, a)
	}
	return filter
}

func (t APIToken) FormatPermissions() string {
	var all []string
	if t.Permissions.Has(APIPermCount) {
		all = append(all, "count")
	}
	if t.Permissions.Has(APIPermExport) {
		all = append(all, "export")
	}
	if t.Permissions.Has(APIPermSiteRead) {
		all = append(all, "site-read")
	}
	if t.Permissions.Has(APIPermSiteCreate) {
		all = append(all, "site-create")
	}
	if t.Permissions.Has(APIPermSiteUpdate) {
		all = append(all, "site-update")
	}
	return "'" + strings.Join(all, "', '") + "'"
}

// Defaults sets fields to default values, unless they're already set.
func (t *APIToken) Defaults(ctx context.Context) {
	t.SiteID = MustGetSite(ctx).ID
	t.Token = zcrypto.Secret256()
	t.CreatedAt = ztime.Now()
}

func (t *APIToken) Validate(ctx context.Context) error {
	v := NewValidate(ctx)
	v.Required("name", t.Name)
	v.Required("site_id", t.SiteID)
	v.Required("user_id", t.SiteID)
	v.Required("token", t.Token)
	if t.Permissions == 1 {
		v.Append("permissions", "must set at least one permission")
	}
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
		`insert into api_tokens (site_id, user_id, name, token, permissions, created_at) values (?)`,
		[]any{t.SiteID, GetUser(ctx).ID, t.Name, t.Token, t.Permissions, t.CreatedAt})
	return errors.Wrap(err, "APIToken.Insert")
}

// Update the name and permissions.
func (t *APIToken) Update(ctx context.Context) error {
	if t.ID == 0 {
		return errors.New("ID == 0")
	}

	t.Defaults(ctx)
	err := t.Validate(ctx)
	if err != nil {
		return err
	}

	err = zdb.Exec(ctx, `update api_tokens set name=?, permissions=? where api_token_id=?`,
		t.Name, t.Permissions, t.ID)
	return errors.Wrap(err, "APIToken.Update")
}

// UpdateLastUsed sets the last used time to the current time.
func (t *APIToken) UpdateLastUsed(ctx context.Context) error {
	if t.ID == 0 {
		return errors.New("ID == 0")
	}

	t.Defaults(ctx)
	err := t.Validate(ctx)
	if err != nil {
		return err
	}

	t.LastUsedAt = ztype.Ptr(ztime.Now())
	err = zdb.Exec(ctx, `update api_tokens set last_used_at=? where api_token_id=?`,
		t.LastUsedAt, t.ID)
	return errors.Wrap(err, "APIToken.UpdateLastUsed")
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

// Find API tokens: by ID if ident is a number, or by token if it's not.
func (t *APITokens) Find(ctx context.Context, ident []string) error {
	ids, strs := splitIntStr(ident)
	err := zdb.Select(ctx, t, `select * from api_tokens where
		{{:ids api_token_id in (:ids) or}}
		{{:strs! 0=1}}
		{{:strs token in (:strs)}}`,
		map[string]any{"ids": ids, "strs": strs})
	return errors.Wrap(err, "APITokens.Find")
}

// IDs gets a list of all IDs for these API tokens.
func (t *APITokens) IDs() []int64 {
	ids := make([]int64, 0, len(*t))
	for _, tt := range *t {
		ids = append(ids, tt.ID)
	}
	return ids
}

// Delete all API tokens in this selection.
func (t *APITokens) Delete(ctx context.Context, _ bool) error {
	err := zdb.TX(ctx, func(ctx context.Context) error {
		for _, tt := range *t {
			err := tt.Delete(WithSite(ctx, &Site{ID: tt.SiteID}))
			if err != nil {
				return err
			}
		}
		return nil
	})
	return errors.Wrap(err, "Users.Delete")
}

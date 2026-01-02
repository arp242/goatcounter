package goatcounter

import (
	"context"
	"fmt"
	"strings"
	"time"

	"zgo.at/errors"
	"zgo.at/guru"
	"zgo.at/z18n"
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

type APITokenID int32

type APIToken struct {
	ID     APITokenID `db:"api_token_id,id" json:"-"`
	SiteID SiteID     `db:"site_id" json:"-"`
	UserID UserID     `db:"user_id" json:"-"`

	Name        string         `db:"name" json:"name"`
	Token       string         `db:"token" json:"-"`
	Permissions zint.Bitflag64 `db:"permissions" json:"permissions"`
	Sites       SiteIDs        `db:"sites" json:"sites"`

	CreatedAt  time.Time  `db:"created_at" json:"-"`
	LastUsedAt *time.Time `db:"last_used_at" json:"-"`
}

func (APIToken) Table() string { return "api_tokens" }

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
	if t.Permissions.Has(APIPermStats) {
		all = append(all, "stats")
	}
	return "'" + strings.Join(all, "', '") + "'"
}

var _ zdb.Defaulter = &APIToken{}

func (t *APIToken) Defaults(ctx context.Context) {
	t.SiteID = MustGetSite(ctx).IDOrParent()
	t.UserID = MustGetUser(ctx).ID
	t.Token = zcrypto.Secret256()
	t.CreatedAt = ztime.Now(ctx)
}

var _ zdb.Validator = &APIToken{}

func (t *APIToken) Validate(ctx context.Context) error {
	v := NewValidate(ctx)
	v.Required("name", t.Name)
	v.Required("site_id", t.SiteID)
	v.Required("user_id", t.SiteID)
	v.Required("token", t.Token)
	if t.Permissions == 1 {
		v.Append("permissions", z18n.T(ctx, "validate/need-one|must select at least one"))
	}
	if len(t.Sites) == 0 {
		v.Append("sites", z18n.T(ctx, "validate/need-one|must select at least one"))
	}
	account := MustGetAccount(ctx)
	if !t.Sites.All() {
		for _, id := range t.Sites {
			var s Site
			err := s.ByID(ctx, id)
			if err != nil {
				return err
			}
			if s.IDOrParent() != account.ID {
				return fmt.Errorf("site %d doesn't not belong to current account %d", s.IDOrParent(), account.ID)
			}
		}
	}
	return v.ErrorOrNil()
}

// Insert a new row.
func (t *APIToken) Insert(ctx context.Context) error {
	err := zdb.Insert(ctx, t)
	return errors.Wrap(err, "APIToken.Insert")
}

// Update the name and permissions.
func (t *APIToken) Update(ctx context.Context) error {
	err := zdb.Update(ctx, t, "name", "permissions")
	return errors.Wrap(err, "APIToken.Update")
}

// Touch sets the last used time to the current time.
func (t *APIToken) Touch(ctx context.Context) error {
	t.LastUsedAt = ztype.Ptr(ztime.Now(ctx))
	err := zdb.Update(ctx, t, "last_used_at")
	return errors.Wrap(err, "APIToken.Touch")
}

func (t *APIToken) ByID(ctx context.Context, id APITokenID) error {
	err := zdb.Get(ctx, t, `/* APIToken.ByID */
		select * from api_tokens where api_token_id=$1 and site_id=$2`,
		id, MustGetSite(ctx).IDOrParent())
	return errors.Wrapf(err, "APIToken.ByID(%d)", id)
}

func (t *APIToken) ByToken(ctx context.Context, token string) error {
	err := zdb.Get(ctx, t,
		`/* APIToken.ByToken */ select * from api_tokens where token=$1 and site_id=$2`,
		token, MustGetSite(ctx).IDOrParent())
	if err != nil {
		return errors.Wrapf(err, "APIToken.ByToken(%q)", token)
	}
	if !t.Sites.Has(MustGetSite(ctx).ID) {
		return guru.New(403, "this token does not have access to this site")
	}
	return nil
}

func (t *APIToken) Delete(ctx context.Context) error {
	err := zdb.Exec(ctx,
		`/* APIToken.Delete */ delete from api_tokens where api_token_id=$1 and site_id=$2`,
		t.ID, MustGetSite(ctx).IDOrParent())
	return errors.Wrapf(err, "APIToken.Delete(%d)", t.ID)
}

type APITokens []APIToken

func (t *APITokens) List(ctx context.Context) error {
	err := zdb.Select(ctx, t,
		`select * from api_tokens where site_id=$1 and user_id=$2`,
		MustGetSite(ctx).IDOrParent(), GetUser(ctx).ID)
	return errors.Wrap(err, "APITokens.List")
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
func (t *APITokens) IDs() []int32 {
	ids := make([]int32, 0, len(*t))
	for _, tt := range *t {
		ids = append(ids, int32(tt.ID))
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

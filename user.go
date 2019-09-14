// Copyright © 2019 Martin Tournoij <martin@arp242.net>
// This file is part of GoatCounter and published under the terms of the AGPLv3,
// which can be found in the LICENSE file or at gnu.org/licenses/agpl.html

package goatcounter

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"net/mail"
	"time"

	"github.com/pkg/errors"
	"github.com/teamwork/guru"
	"github.com/teamwork/utils/jsonutil"
	"github.com/teamwork/validate"
	"zgo.at/zhttp"
	"zgo.at/zhttp/ctxkey"
	"zgo.at/zlog"

	"zgo.at/goatcounter/cfg"
	"zgo.at/goatcounter/smail"
)

const (
	UserRoleRegular = ""
	UserRoleAdmin   = "a"
)

// User entry.
type User struct {
	ID   int64 `db:"id" json:"-"`
	Site int64 `db:"site" json:"-"`

	Name        string          `db:"name" json:"name"`
	Email       string          `db:"email" json:"email"`
	Role        string          `db:"role" json:"-"`
	LoginReq    *time.Time      `db:"login_req" json:"-"`
	LoginKey    *string         `db:"login_key" json:"-"`
	CSRFToken   *string         `db:"csrf_token" json:"-"`
	Preferences UserPreferences `db:"preferences" json:"preferences"`

	State     string     `db:"state" json:"-"`
	CreatedAt time.Time  `db:"created_at" json:"-"`
	UpdatedAt *time.Time `db:"updated_at" json:"-"`
}

type UserPreferences struct {
	DateFormat string `json:"date_format"`
}

func (up UserPreferences) String() string { return string(jsonutil.MustMarshal(up)) }

// Value implements the SQL Value function to determine what to store in the DB.
func (up UserPreferences) Value() (driver.Value, error) { return json.Marshal(up) }

// Scan converts the data returned from the DB into the struct.
func (up *UserPreferences) Scan(v interface{}) error {
	switch vv := v.(type) {
	case []byte:
		return json.Unmarshal(vv, up)
	case string:
		return json.Unmarshal([]byte(vv), up)
	default:
		panic(fmt.Sprintf("unsupported type: %T", v))
	}
}

// Defaults sets fields to default values, unless they're already set.
func (u *User) Defaults(ctx context.Context) {
	// TODO: not set in website
	// site := MustGetSite(ctx)
	// u.Site = site.ID

	if u.State == "" {
		u.State = StateRequest
	}

	if u.CreatedAt.IsZero() {
		u.CreatedAt = time.Now().UTC()
	} else {
		t := time.Now().UTC()
		u.UpdatedAt = &t
	}
}

// Validate the object.
func (u *User) Validate(ctx context.Context) error {
	v := validate.New()

	v.Required("site", u.Site)
	v.Required("name", u.Name)
	v.Required("email", u.Email)

	v.Len("name", u.Name, 1, 200)
	v.Len("email", u.Email, 5, 255)
	v.Email("email", u.Email)
	v.Include("state", u.State, States)

	return v.ErrorOrNil()
}

// Insert a new row.
func (u *User) Insert(ctx context.Context) error {
	if u.ID > 0 {
		return errors.New("ID > 0")
	}

	u.Defaults(ctx)
	err := u.Validate(ctx)
	if err != nil {
		return err
	}

	res, err := MustGetDB(ctx).ExecContext(ctx,
		`insert into users (site, name, email, created_at) values ($1, $2, $3, $4)`,
		u.Site, u.Name, u.Email, sqlDate(u.CreatedAt))
	if err != nil {
		if uniqueErr(err) {
			return guru.New(400, "this user already exists")
		}
		return errors.Wrap(err, "User.Insert")
	}

	if cfg.PgSQL {
		var nu User
		err = nu.ByEmail(context.WithValue(ctx, ctxkey.Site, &Site{ID: u.Site}), u.Email)
		u.ID = nu.ID
	} else {
		u.ID, err = res.LastInsertId()
	}
	return nil
}

// ByID gets a user by ID.
func (u *User) ByID(ctx context.Context, id int64) error {
	return errors.Wrap(MustGetDB(ctx).GetContext(ctx, u,
		`select * from users where id=$1 and site=$2 and state=$3`,
		id, MustGetSite(ctx).ID, StateActive), "User.ByID")
}

// ByEmail gets a user by email address.
func (u *User) ByEmail(ctx context.Context, email string) error {
	return errors.Wrap(MustGetDB(ctx).GetContext(ctx, u,
		`select * from users where
			lower(email)=lower($1) and
			state=$2 and
			(site=$3 or site=(select parent from sites where id=$3))
		`, email, StateActive, MustGetSite(ctx).ID), "User.ByEmail")
}

// ByKey gets a user by login key.
func (u *User) ByKey(ctx context.Context, key string) error {
	if key == "" { // Quick exit when called from zhttp.Auth()
		return sql.ErrNoRows
	}

	query := `select users.* from users
		where login_key=$1 and
			users.site=$2 and
			users.state=$3 and
			(login_req is null or `

	if cfg.PgSQL {
		query += `login_req + interval '15 minutes' > now())`
	} else {
		query += `datetime(login_req, '+15 minutes') > datetime())`
	}

	return errors.Wrap(MustGetDB(ctx).GetContext(ctx, u, query,
		key, MustGetSite(ctx).IDOrParent(), StateActive), "User.ByKey")
}

// RequestLogin generates a new login Key.
func (u *User) RequestLogin(ctx context.Context) error {
	u.LoginKey = zhttp.SecretP()

	_, err := MustGetDB(ctx).ExecContext(ctx, `update users set
		login_key=$1, login_req=current_timestamp
		where id=$2 and site=$3`, *u.LoginKey, u.ID, MustGetSite(ctx).IDOrParent())
	return errors.Wrap(err, "User.RequestLogin")
}

// Login a user; create a new key, CSRF token, and reset the request date.
func (u *User) Login(ctx context.Context) error {
	u.LoginKey = zhttp.SecretP()
	u.CSRFToken = zhttp.SecretP()

	_, err := MustGetDB(ctx).ExecContext(ctx, `update users set
			login_key=$1, login_req=null, csrf_token=$2
			where id=$3 and site=$4`,
		*u.LoginKey, *u.CSRFToken, u.ID, MustGetSite(ctx).IDOrParent())
	return errors.Wrap(err, "User.Login")
}

func (u *User) Logout(ctx context.Context) error {
	_, err := MustGetDB(ctx).ExecContext(ctx,
		`update users set login_key=null, login_req=null where id=$1 and site=$2`,
		u.ID, MustGetSite(ctx).IDOrParent())
	return errors.Wrap(err, "User.Logout")
}

func (u *User) GetToken() string {
	if u.CSRFToken == nil {
		return ""
	}
	return *u.CSRFToken
}

func (u *User) SendLoginMail(ctx context.Context, site Site) {
	var url = fmt.Sprintf("%s.%s/user/login/%s", site.Code, cfg.Domain, *u.LoginKey)
	go func() {
		err := smail.Send("Your login URL",
			mail.Address{Name: "GoatCounter login", Address: "login@goatcounter.com"},
			[]mail.Address{{Name: u.Name, Address: u.Email}},
			fmt.Sprintf("Hi there,\n\nYour login URL for Goatcounter is:\n\n  https://%s\n\nGo to it to log in.\n",
				url))
		if err != nil {
			zlog.Errorf("smail: %s", err)
		}
	}()
}

type Users []User

func (u *Users) ListAllSites(ctx context.Context) error {
	return errors.Wrap(MustGetDB(ctx).SelectContext(ctx, u,
		`select * from users order by created_at desc`),
		"Users.List")
}

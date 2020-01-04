// Copyright © 2019 Martin Tournoij <martin@arp242.net>
// This file is part of GoatCounter and published under the terms of the EUPL
// v1.2, which can be found in the LICENSE file or at http://eupl12.zgo.at

package goatcounter

import (
	"context"
	"database/sql"
	"fmt"
	"net/mail"
	"time"

	"github.com/pkg/errors"
	"github.com/teamwork/guru"
	"zgo.at/goatcounter/cfg"
	"zgo.at/zdb"
	"zgo.at/zhttp"
	"zgo.at/zhttp/ctxkey"
	"zgo.at/zhttp/zmail"
	"zgo.at/zlog"
	"zgo.at/zvalidate"
)

const (
	UserRoleRegular = ""
	UserRoleAdmin   = "a"
)

// User entry.
type User struct {
	ID   int64 `db:"id" json:"-"`
	Site int64 `db:"site" json:"-"`

	Name          string     `db:"name" json:"name"`
	Email         string     `db:"email" json:"email"`
	Role          string     `db:"role" json:"-"`
	LoginAt       *time.Time `db:"login_at" json:"-"`
	LoginRequest  *string    `db:"login_request" json:"-"`
	LoginToken    *string    `db:"login_token" json:"-"`
	CSRFToken     *string    `db:"csrf_token" json:"-"`
	SeenUpdatesAt time.Time  `db:"seen_updates_at" json:"-"`

	CreatedAt time.Time  `db:"created_at" json:"-"`
	UpdatedAt *time.Time `db:"updated_at" json:"-"`
}

// Defaults sets fields to default values, unless they're already set.
func (u *User) Defaults(ctx context.Context) {
	if s := GetSite(ctx); s != nil && s.ID > 0 { // Not set in website.
		u.Site = s.ID
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
	v := zvalidate.New()

	v.Required("site", u.Site)
	v.Required("name", u.Name)
	v.Required("email", u.Email)

	v.Len("name", u.Name, 1, 200)
	v.Len("email", u.Email, 5, 255)
	v.Email("email", u.Email)

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

	res, err := zdb.MustGet(ctx).ExecContext(ctx,
		`insert into users (site, name, email, created_at) values ($1, $2, $3, $4)`,
		u.Site, u.Name, u.Email, u.CreatedAt.Format(zdb.Date))
	if err != nil {
		if zdb.UniqueErr(err) {
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
	return errors.Wrap(err, "User.Insert")
}

// Update this user's name and email.
func (u *User) Update(ctx context.Context) error {
	if u.ID == 0 {
		return errors.New("ID == 0")
	}

	u.Defaults(ctx)
	err := u.Validate(ctx)
	if err != nil {
		return err
	}

	_, err = zdb.MustGet(ctx).ExecContext(ctx,
		`update users set name=$1, email=$2, updated_at=$3 where id=$4`,
		u.Name, u.Email, u.UpdatedAt.Format(zdb.Date), u.ID)
	return errors.Wrap(err, "User.Update")
}

// ByEmail gets a user by email address.
func (u *User) ByEmail(ctx context.Context, email string) error {
	return errors.Wrap(zdb.MustGet(ctx).GetContext(ctx, u,
		`select * from users where
			lower(email)=lower($1) and
			(site=$2 or site=(select parent from sites where id=$2))
		`, email, MustGetSite(ctx).ID), "User.ByEmail")
}

// ByLoginRequest gets a user by login request key.
func (u *User) ByLoginRequest(ctx context.Context, key string) error {
	if key == "" { // Quick exit when called from zhttp.Auth()
		return sql.ErrNoRows
	}

	query := `select users.* from users
		where login_request=$1 and users.site=$2 and `

	if cfg.PgSQL {
		query += `login_at + interval '15 minutes' > now()`
	} else {
		query += `datetime(login_at, '+15 minutes') > datetime()`
	}

	return errors.Wrap(zdb.MustGet(ctx).GetContext(ctx, u, query,
		key, MustGetSite(ctx).IDOrParent()), "User.ByLoginRequest")
}

// ByToken gets a user by login token.
func (u *User) ByToken(ctx context.Context, token string) error {
	if token == "" { // Quick exit when called from zhttp.Auth()
		return sql.ErrNoRows
	}

	return errors.Wrap(zdb.MustGet(ctx).GetContext(ctx, u, `
		select users.* from users where login_token=$1`,
		token), "User.ByToken")
}

// BySite gets a user by site.
func (u *User) BySite(ctx context.Context, id int64) error {
	var s Site
	err := s.ByID(ctx, id)
	if err != nil {
		return err
	}

	return errors.Wrap(zdb.MustGet(ctx).GetContext(ctx, u,
		`select * from users where site=$1`, s.IDOrParent()), "User.ByID")
}

// RequestLogin generates a new login Key.
func (u *User) RequestLogin(ctx context.Context) error {
	u.LoginRequest = zhttp.SecretP()
	_, err := zdb.MustGet(ctx).ExecContext(ctx, `update users set
		login_request=$1, login_at=current_timestamp
		where id=$2 and site=$3`, *u.LoginRequest, u.ID, MustGetSite(ctx).IDOrParent())
	return errors.Wrap(err, "User.RequestLogin")
}

// Login a user; create a new key, CSRF token, and reset the request date.
//
// How logins work:
//
//   1. login_request is set to a temporary token that expires in 15 mins.
//   2. User goes to /user/login/<login_request> via email.
//   3. If there already is a login_token, set the cookie to that. Otherwise
//      generate a new one.
func (u *User) Login(ctx context.Context) error {
	u.CSRFToken = zhttp.SecretP()
	if u.LoginToken == nil {
		s := time.Now().Format("20060102") + "-" + zhttp.Secret()
		u.LoginToken = &s
	}

	_, err := zdb.MustGet(ctx).ExecContext(ctx, `update users set
			login_request=null, login_token=$1, csrf_token=$2
			where id=$3 and site=$4`,
		u.LoginToken, u.CSRFToken, u.ID, MustGetSite(ctx).IDOrParent())
	return errors.Wrap(err, "User.Login")
}

// Logout a user.
func (u *User) Logout(ctx context.Context) error {
	u.LoginToken = nil
	u.LoginRequest = nil
	u.LoginAt = nil
	_, err := zdb.MustGet(ctx).ExecContext(ctx,
		`update users set login_token=null, login_request=null where id=$1 and site=$2`,
		u.ID, MustGetSite(ctx).IDOrParent())
	return errors.Wrap(err, "User.Logout")
}

// GetToken gets the CSRF token.
func (u *User) GetToken() string {
	if u.CSRFToken == nil {
		return ""
	}
	return *u.CSRFToken
}

// SendLoginMail sends the login email.
func (u *User) SendLoginMail(ctx context.Context, site *Site) {
	go func() {
		err := zmail.Send("Your login URL",
			mail.Address{Name: "GoatCounter login", Address: "login@goatcounter.com"},
			[]mail.Address{{Name: u.Name, Address: u.Email}},
			fmt.Sprintf("Hi there,\n\nYour login URL for Goatcounter is:\n\n  %s/user/login/%s\n\nGo to it to log in.\n",
				site.URL(), *u.LoginRequest))
		if err != nil {
			zlog.Errorf("zmail: %s", err)
		}
	}()
}

// SeenUpdates marks this user as having seen all updates up until now.
func (u *User) SeenUpdates(ctx context.Context) error {
	u.SeenUpdatesAt = time.Now().UTC()
	_, err := zdb.MustGet(ctx).ExecContext(ctx,
		`update users set seen_updates_at=$1 where id=$2`, u.SeenUpdatesAt, u.ID)
	return errors.Wrap(err, "User.SeenUpdatesAt")
}

type Users []User

// ByEmail gets all users with this email address.
func (u *Users) ByEmail(ctx context.Context, email string) error {
	return errors.Wrap(zdb.MustGet(ctx).SelectContext(ctx, u,
		`select * from users where lower(email)=lower($1) order by id asc`, email),
		"Users.ByEmail")
}

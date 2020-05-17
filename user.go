// Copyright © 2019 Martin Tournoij <martin@arp242.net>
// This file is part of GoatCounter and published under the terms of the EUPL
// v1.2, which can be found in the LICENSE file or at http://eupl12.zgo.at

package goatcounter

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"golang.org/x/crypto/bcrypt"
	"zgo.at/errors"
	"zgo.at/goatcounter/cfg"
	"zgo.at/guru"
	"zgo.at/zdb"
	"zgo.at/zhttp"
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

	Email         string     `db:"email" json:"email"`
	EmailVerified zdb.Bool   `db:"email_verified" json:"-"`
	Password      []byte     `db:"password" json:"-"`
	Role          string     `db:"role" json:"-"`
	LoginAt       *time.Time `db:"login_at" json:"-"`
	ResetAt       *time.Time `db:"reset_at" json:"-"`
	LoginRequest  *string    `db:"login_request" json:"-"`
	LoginToken    *string    `db:"login_token" json:"-"`
	CSRFToken     *string    `db:"csrf_token" json:"-"`
	EmailToken    *string    `db:"email_token" json:"-"`
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
		u.CreatedAt = Now()
	} else {
		t := Now()
		u.UpdatedAt = &t
	}

	if !u.EmailVerified {
		u.EmailToken = zhttp.SecretP()
	}
}

// Validate the object.
func (u *User) Validate(ctx context.Context, validatePassword bool) error {
	v := zvalidate.New()

	v.Required("site", u.Site)
	v.Required("email", u.Email)
	v.Len("email", u.Email, 5, 255)
	v.Email("email", u.Email)

	if validatePassword {
		sp := string(u.Password)
		v.Required("password", u.Password)
		v.UTF8("password", sp)
		if len(sp) < 8 || len(sp) > 50 {
			v.Append("password", "must be between 8 and 50 bytes")
		}
	}

	return v.ErrorOrNil()
}

// Hash the password, replacing the plain-text one.
func (u *User) hashPassword() error {
	// Length is capped to 50 characters in Validate.
	if len(u.Password) > 50 {
		return fmt.Errorf("User.hashPassword: already hashed")
	}

	pwd, err := bcrypt.GenerateFromPassword(u.Password, bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("User.hashPassword: %w", err)
	}
	u.Password = pwd
	return nil
}

// Insert a new row.
func (u *User) Insert(ctx context.Context) error {
	if u.ID > 0 {
		return errors.New("ID > 0")
	}

	u.Defaults(ctx)
	err := u.Validate(ctx, true)
	if err != nil {
		return err
	}

	err = u.hashPassword()
	if err != nil {
		return errors.Wrap(err, "User.Insert")
	}

	query := `insert into users `
	args := []interface{}{u.Site, u.Email, u.Password, u.CreatedAt.Format(zdb.Date)}
	if u.EmailVerified {
		query += ` (site, email, password, created_at, email_verified) values ($1, $2, $3, $4, 1)`
	} else {
		query += ` (site, email, password, created_at, email_token) values ($1, $2, $3, $4, $5)`
		args = append(args, u.EmailToken)
	}

	res, err := zdb.MustGet(ctx).ExecContext(ctx, query, args...)
	if err != nil {
		if zdb.ErrUnique(err) {
			return guru.New(400, "this user already exists")
		}
		return errors.Wrap(err, "User.Insert")
	}

	if cfg.PgSQL {
		var nu User
		// No site yet when signing up since it's on www.goatcounter.com
		err = nu.ByEmail(WithSite(ctx, &Site{ID: u.Site}), u.Email)
		u.ID = nu.ID
	} else {
		u.ID, err = res.LastInsertId()
	}

	return errors.Wrap(err, "User.Insert: get ID")
}

// Update this user's name, email.
func (u *User) Update(ctx context.Context, emailChanged bool) error {
	if u.ID == 0 {
		return errors.New("ID == 0")
	}

	u.Defaults(ctx)
	err := u.Validate(ctx, false)
	if err != nil {
		return err
	}

	if emailChanged {
		u.EmailVerified = false
		u.EmailToken = zhttp.SecretP()
	}

	_, err = zdb.MustGet(ctx).ExecContext(ctx,
		`update users set email=$1, updated_at=$2, email_verified=$3, email_token=$4 where id=$5`,
		u.Email, u.UpdatedAt.Format(zdb.Date), u.EmailVerified, u.EmailToken, u.ID)
	return errors.Wrap(err, "User.Update")
}

// UpdatePassword updates this user's password.
func (u *User) UpdatePassword(ctx context.Context, pwd string) error {
	if u.ID == 0 {
		return errors.New("ID == 0")
	}

	u.Password = []byte(pwd)
	u.Defaults(ctx)
	err := u.Validate(ctx, true)
	if err != nil {
		return err
	}

	err = u.hashPassword()
	if err != nil {
		return errors.Wrap(err, "User.UpdatePassword")
	}

	_, err = zdb.MustGet(ctx).ExecContext(ctx,
		`update users set password=$1, updated_at=$2 where id=$3`,
		u.Password, u.UpdatedAt.Format(zdb.Date), u.ID)
	return errors.Wrap(err, "User.UpdatePassword")
}

// CorrectPassword verifies that this password is correct.
func (u User) CorrectPassword(pwd string) (bool, error) {
	err := bcrypt.CompareHashAndPassword(u.Password, []byte(pwd))
	if errors.Is(err, bcrypt.ErrMismatchedHashAndPassword) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("user.CorrectPassword: %w", err)
	}
	return true, nil
}

func (u *User) VerifyEmail(ctx context.Context) error {
	_, err := zdb.MustGet(ctx).ExecContext(ctx,
		`update users set email_verified=1, email_token=null where id=$1`,
		u.ID)
	return errors.Wrap(err, "User.VerifyEmail")
}

// ByEmailToken gets a user by email verification token.
func (u *User) ByEmailToken(ctx context.Context, key string) error {
	return errors.Wrap(zdb.MustGet(ctx).GetContext(ctx, u,
		`select * from users where site=$1 and email_token=$2`,
		MustGetSite(ctx).IDOrParent(), key), "User.ByEmailToken")
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

	query := `select * from users
		where login_request=$1 and site=$2 and `

	if cfg.PgSQL {
		query += `login_at + interval '60 minutes' > now()`
	} else {
		query += `datetime(login_at, '+60 minutes') > datetime()`
	}

	return errors.Wrap(zdb.MustGet(ctx).GetContext(ctx, u, query,
		key, MustGetSite(ctx).IDOrParent()), "User.ByLoginRequest")
}

// ByResetToken gets a user by login request key.
func (u *User) ByResetToken(ctx context.Context, key string) error {
	query := `select * from users
		where login_request=$1 and site=$2 and `

	if cfg.PgSQL {
		query += `reset_at + interval '60 minutes' > now()`
	} else {
		query += `datetime(reset_at, '+60 minutes') > datetime()`
	}

	return errors.Wrap(zdb.MustGet(ctx).GetContext(ctx, u, query,
		key, MustGetSite(ctx).IDOrParent()), "User.ByResetToken")
}

// ByToken gets a user by login token.
func (u *User) ByToken(ctx context.Context, token string) error {
	if token == "" {
		return sql.ErrNoRows
	}

	return errors.Wrap(zdb.MustGet(ctx).GetContext(ctx, u,
		`select * from users where login_token=$1`, token),
		"User.ByToken")
}

// ByTokenAndSite gets a user by login token.
func (u *User) ByTokenAndSite(ctx context.Context, token string) error {
	if token == "" {
		return sql.ErrNoRows
	}

	return errors.Wrap(zdb.MustGet(ctx).GetContext(ctx, u,
		`select * from users where login_token=$1 and site=$2`,
		token, MustGetSite(ctx).IDOrParent()), "User.ByTokenAndSite")
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

// RequestReset generates a new password reset key.
func (u *User) RequestReset(ctx context.Context) error {
	// TODO: rename
	// Recycle the request_login for now; will rename after removing email auth.
	u.LoginRequest = zhttp.SecretP()
	_, err := zdb.MustGet(ctx).ExecContext(ctx, `update users set
		login_request=$1, reset_at=current_timestamp where id=$2 and site=$3`,
		*u.LoginRequest, u.ID, MustGetSite(ctx).IDOrParent())
	return errors.Wrap(err, "User.RequestReset")
}

// Login a user; create a new key, CSRF token, and reset the request date.
func (u *User) Login(ctx context.Context) error {
	u.CSRFToken = zhttp.SecretP()
	if u.LoginToken == nil {
		s := Now().Format("20060102") + "-" + zhttp.Secret()
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

// SeenUpdates marks this user as having seen all updates up until now.
func (u *User) SeenUpdates(ctx context.Context) error {
	u.SeenUpdatesAt = Now()
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

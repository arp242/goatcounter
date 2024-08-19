// Copyright © Martin Tournoij – This file is part of GoatCounter and published
// under the terms of a slightly modified EUPL v1.2 license, which can be found
// in the LICENSE file or at https://license.goatcounter.com

package goatcounter

import (
	"context"
	"crypto/rand"
	"database/sql"
	"database/sql/driver"
	"fmt"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
	"zgo.at/errors"
	"zgo.at/guru"
	"zgo.at/json"
	"zgo.at/zdb"
	"zgo.at/zlog"
	"zgo.at/zstd/zbool"
	"zgo.at/zstd/zcrypto"
	"zgo.at/zstd/ztime"
	"zgo.at/zstd/ztype"
)

const totpSecretLen = 16

// User entry.
type User struct {
	ID   int64 `db:"user_id" json:"id,readonly"`
	Site int64 `db:"site_id" json:"site,readonly"`

	Email         string       `db:"email" json:"email"`
	EmailVerified zbool.Bool   `db:"email_verified" json:"email_verified,readonly"`
	Password      []byte       `db:"password" json:"-"`
	TOTPEnabled   zbool.Bool   `db:"totp_enabled" json:"totp_enabled,readonly"`
	TOTPSecret    []byte       `db:"totp_secret" json:"-"`
	Access        UserAccesses `db:"access" json:"access,readonly"`
	LoginAt       *time.Time   `db:"login_at" json:"login_at,readonly"`
	OpenAt        *time.Time   `db:"open_at" json:"open_at,readonly"`
	ResetAt       *time.Time   `db:"reset_at" json:"reset_at,readonly"`
	LoginRequest  *string      `db:"login_request" json:"-"`
	LoginToken    *string      `db:"login_token" json:"-"`
	Token         *string      `db:"csrf_token" json:"-"`
	EmailToken    *string      `db:"email_token" json:"-"`
	Settings      UserSettings `db:"settings" json:"settings"`

	// Keep track when the last email report was sent, so we don't double-send them.
	LastReportAt time.Time `db:"last_report_at" json:"last_report_at"`

	CreatedAt time.Time  `db:"created_at" json:"created_at,readonly"`
	UpdatedAt *time.Time `db:"updated_at" json:"updated_at,readonly"`
}

// Defaults sets fields to default values, unless they're already set.
func (u *User) Defaults(ctx context.Context) {
	if s := GetSite(ctx); s != nil && s.ID > 0 { // Not set in website.
		u.Site = s.IDOrParent()
	}

	if u.CreatedAt.IsZero() {
		u.CreatedAt = ztime.Now()
	} else {
		t := ztime.Now()
		u.UpdatedAt = &t
	}

	if u.LastReportAt.IsZero() {
		u.LastReportAt = ztime.Now()
	}

	if !u.EmailVerified {
		u.EmailToken = ztype.Ptr[string](zcrypto.Secret192())
	}

	u.Settings.Defaults(ctx)
}

// Validate the object.
func (u *User) Validate(ctx context.Context, validatePassword bool) error {
	v := NewValidate(ctx)

	v.Required("site", u.Site)
	v.Required("email", u.Email)
	v.Len("email", u.Email, 5, 255)
	v.Email("email", u.Email)
	if len(u.Access) == 0 {
		v.Append("access", "must be set")
	}

	if validatePassword {
		sp := string(u.Password)
		v.Required("password", u.Password)
		v.UTF8("password", sp)
		if len(sp) < 8 || len(sp) > 50 {
			v.Append("password", "must be between 8 and 50 bytes")
		}
	}

	v.Sub("settings", "", u.Settings.Validate(ctx))

	return v.ErrorOrNil()
}

// Hash the password, replacing the plain-text one.
func (u *User) hashPassword(ctx context.Context) error {
	// Length is capped to 50 characters in Validate.
	if len(u.Password) > 50 {
		return errors.Errorf("User.hashPassword: already hashed")
	}

	cost := bcrypt.DefaultCost
	if Config(ctx).BcryptMinCost { // Otherwise every test take 1.5s extra
		cost = bcrypt.MinCost
	}
	pwd, err := bcrypt.GenerateFromPassword(u.Password, cost)
	if err != nil {
		return errors.Errorf("User.hashPassword: %w", err)
	}
	u.Password = pwd
	return nil
}

// Insert a new row.
func (u *User) Insert(ctx context.Context, allowBlankPassword bool) error {
	if u.ID > 0 {
		return errors.New("ID > 0")
	}

	hasPassword := !(u.Password == nil && allowBlankPassword)

	u.Defaults(ctx)
	err := u.Validate(ctx, hasPassword)
	if err != nil {
		return err
	}

	if hasPassword {
		err = u.hashPassword(ctx)
		if err != nil {
			return errors.Wrap(err, "User.Insert")
		}
	}

	u.TOTPEnabled = zbool.Bool(false)
	u.TOTPSecret = make([]byte, totpSecretLen)
	_, err = rand.Read(u.TOTPSecret)
	if err != nil {
		return errors.Wrap(err, "User.Insert")
	}

	query := `insert into users `
	args := []any{u.Site, u.Email, u.Password, u.TOTPSecret, u.Settings, u.Access, u.CreatedAt, u.LastReportAt}
	if u.EmailVerified {
		query += ` (site_id, email, password, totp_secret, settings, access, created_at, last_report_at, email_verified) values (?)`
		args = append(args, 1)
	} else {
		query += ` (site_id, email, password, totp_secret, settings, access, created_at, last_report_at, email_token) values (?)`
		args = append(args, u.EmailToken)
	}

	u.ID, err = zdb.InsertID(ctx, "user_id", query, args)
	if err != nil {
		if zdb.ErrUnique(err) {
			return guru.New(400, "this user already exists")
		}
		return errors.Wrap(err, "User.Insert")
	}
	return nil
}

// Delete this user.
func (u *User) Delete(ctx context.Context, lastAdmin bool) error {
	if !lastAdmin {
		var admins Users
		err := admins.BySite(ctx, u.Site)
		if err != nil {
			return errors.Wrap(err, "User.Delete")
		}
		admins = admins.Admins()
		if len(admins) == 1 && admins[0].ID == u.ID {
			return fmt.Errorf("can't delete last admin user for site %d", u.Site)
		}
	}

	account, err := GetAccount(ctx)
	if err != nil {
		return errors.Wrap(err, "User.Delete")
	}

	err = zdb.Exec(ctx, `delete from users where user_id=? and site_id=?`,
		u.ID, account.ID)
	return errors.Wrap(err, "User.Delete")
}

// Update this user's name, email, settings, and access.
func (u *User) Update(ctx context.Context, emailChanged bool) error {
	if u.ID == 0 {
		return errors.New("ID == 0")
	}

	u.Defaults(ctx)
	err := u.Validate(ctx, false)
	if err != nil {
		return err
	}

	if emailChanged && Config(ctx).GoatcounterCom {
		u.EmailVerified = false
		u.EmailToken = ztype.Ptr(zcrypto.Secret192())
	}

	account, err := GetAccount(ctx)
	if err != nil {
		return errors.Wrap(err, "User.Update")
	}

	err = zdb.Exec(ctx, `update users
		set email=?, settings=?, access=?, updated_at=?, email_verified=?, email_token=?, last_report_at=?
		where user_id=? and site_id=?`,
		u.Email, u.Settings, u.Access, u.UpdatedAt, u.EmailVerified, u.EmailToken, u.LastReportAt, u.ID, account.ID)
	return errors.Wrap(err, "User.Update")
}

// UpdateSite updates this user's siteID (i.e. moves it to another site).
func (u *User) UpdateSite(ctx context.Context) error {
	if u.ID == 0 {
		return errors.New("ID == 0")
	}

	u.Defaults(ctx)
	err := u.Validate(ctx, false)
	if err != nil {
		return err
	}

	err = zdb.Exec(ctx, `update users set site_id=? where user_id=?`, u.Site, u.ID)
	return errors.Wrap(err, "User.UpdateSite")
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

	err = u.hashPassword(ctx)
	if err != nil {
		return errors.Wrap(err, "User.UpdatePassword")
	}

	err = zdb.Exec(ctx,
		`update users set password=$1, updated_at=$2 where user_id=$3`,
		u.Password, u.UpdatedAt, u.ID)
	return errors.Wrap(err, "User.UpdatePassword")
}

// CorrectPassword verifies that this password is correct.
func (u User) CorrectPassword(pwd string) (bool, error) {
	err := bcrypt.CompareHashAndPassword(u.Password, []byte(pwd))
	if errors.Is(err, bcrypt.ErrMismatchedHashAndPassword) {
		return false, nil
	}
	if err != nil {
		return false, errors.Errorf("user.CorrectPassword: %w", err)
	}
	return true, nil
}

func (u *User) VerifyEmail(ctx context.Context) error {
	err := zdb.Exec(ctx,
		`update users set email_verified=1, email_token=null where user_id=$1`,
		u.ID)
	return errors.Wrap(err, "User.VerifyEmail")
}

// ByEmailToken gets a user by email verification token.
func (u *User) ByEmailToken(ctx context.Context, key string) error {
	return errors.Wrap(zdb.Get(ctx, u,
		`select * from users where site_id=$1 and email_token=$2`,
		MustGetSite(ctx).IDOrParent(), key), "User.ByEmailToken")
}

// ByID gets a user by id.
func (u *User) ByID(ctx context.Context, id int64) error {
	err := zdb.Get(ctx, u, `select * from users where user_id=? and site_id=?`,
		id, MustGetSite(ctx).IDOrParent())
	return errors.Wrap(err, "User.ByID")
}

// ByEmail gets a user by email address.
func (u *User) ByEmail(ctx context.Context, email string) error {
	err := zdb.Get(ctx, u, `select * from users where lower(email) = lower(?) and site_id = ?`,
		email, MustGetSite(ctx).IDOrParent())
	return errors.Wrap(err, "User.ByEmail")
}

// Find a user: by ID if ident is a number, or by email if it's not.
func (u *User) Find(ctx context.Context, ident string) error {
	id, err := strconv.ParseInt(ident, 10, 64)
	if err == nil {
		return errors.Wrap(u.ByID(ctx, id), "User.Find")
	}

	s, email, _ := strings.Cut(ident, ",")

	var site Site
	err = site.Find(ctx, s)
	if err != nil {
		return errors.Wrap(err, "User.Find")
	}

	err = u.ByEmail(WithSite(ctx, &site), email)
	return errors.Wrap(err, "User.Find")
}

// ByResetToken gets a user by login request key.
//
// This can be used in two contexts: the user requested a password reset, or the
// user was invited to create a new account.
func (u *User) ByResetToken(ctx context.Context, key string) error {
	timeout := "2 hours"
	if strings.HasPrefix(key, "invite-") {
		timeout = "168 hours"
	}

	query := `select * from users where login_request=$1 and site_id=$2 and `
	if zdb.SQLDialect(ctx) == zdb.DialectPostgreSQL {
		query += fmt.Sprintf(`reset_at + interval '%s' > now()`, timeout)
	} else {
		query += fmt.Sprintf(`datetime(reset_at, '+%s') > datetime()`, timeout)
	}

	return errors.Wrap(zdb.Get(ctx, u, query,
		key, MustGetSite(ctx).IDOrParent()), "User.ByResetToken")
}

// ByToken gets a user by login token.
func (u *User) ByToken(ctx context.Context, token string) error {
	if token == "" {
		return sql.ErrNoRows
	}

	return errors.Wrap(zdb.Get(ctx, u,
		`select * from users where login_token=$1`, token),
		"User.ByToken")
}

// ByTokenAndSite gets a user by login token.
func (u *User) ByTokenAndSite(ctx context.Context, token string) error {
	if token == "" {
		return sql.ErrNoRows
	}

	return errors.Wrap(zdb.Get(ctx, u,
		`select * from users where login_token=$1 and site_id=$2`,
		token, MustGetSite(ctx).IDOrParent()), "User.ByTokenAndSite")
}

// RequestReset generates a new password reset key.
func (u *User) RequestReset(ctx context.Context) error {
	// TODO: rename this, as it's now used for password resets.
	u.LoginRequest = ztype.Ptr(zcrypto.Secret128())
	err := zdb.Exec(ctx, `update users set
		login_request=$1, reset_at=current_timestamp where user_id=$2 and site_id=$3`,
		*u.LoginRequest, u.ID, MustGetSite(ctx).IDOrParent())
	return errors.Wrap(err, "User.RequestReset")
}

func (u *User) InviteToken(ctx context.Context) error {
	u.LoginRequest = ztype.Ptr("invite-" + zcrypto.Secret128())
	err := zdb.Exec(ctx, `update users set
		login_request=$1, reset_at=current_timestamp where user_id=$2 and site_id=$3`,
		*u.LoginRequest, u.ID, MustGetSite(ctx).IDOrParent())
	return errors.Wrap(err, "User.RequestReset")
}

func (u *User) EnableTOTP(ctx context.Context) error {
	err := zdb.Exec(ctx, `update users set totp_enabled=1 where user_id=$1 and site_id=$2`,
		u.ID, MustGetSite(ctx).IDOrParent())
	if err != nil {
		return errors.Wrap(err, "User.EnableTOTP")
	}
	u.TOTPEnabled = zbool.Bool(true)
	return nil
}

func (u *User) DisableTOTP(ctx context.Context) error {
	// Reset the totp secret to something new so that we don't end up re-using the
	// old secret by mistake and so that we're sure that it's invalidated.
	secret := make([]byte, totpSecretLen)
	_, err := rand.Read(secret)
	if err != nil {
		return errors.Wrap(err, "User.DisableTOTP")
	}

	err = zdb.Exec(ctx, `update users set
		totp_enabled=0, totp_secret=$1 where user_id=$2 and site_id=$3`,
		secret, u.ID, MustGetSite(ctx).IDOrParent())
	if err != nil {
		return errors.Wrap(err, "User.DisableTOTP")
	}
	u.TOTPSecret = secret
	u.TOTPEnabled = zbool.Bool(false)
	return nil
}

// Login a user; create a new key, CSRF token, and reset the request date.
func (u *User) Login(ctx context.Context) error {
	if u.ID == 0 {
		return errors.New("u.ID == 0")
	}

	u.Token = ztype.Ptr(zcrypto.Secret256())
	if u.LoginToken == nil || *u.LoginToken == "" {
		s := ztime.Now().Format("20060102") + "-" + zcrypto.Secret256()
		u.LoginToken = &s
	}

	u.LoginAt = ztype.Ptr(ztime.Now())
	u.OpenAt = ztype.Ptr(ztime.Now())
	err := zdb.Exec(ctx, `update users set
			login_request=null, login_token=?, csrf_token=?, login_at=?, open_at=?
			where user_id = ? and site_id = ?`,
		u.LoginToken, u.Token, u.LoginAt, u.OpenAt,
		u.ID, MustGetSite(ctx).IDOrParent())
	return errors.Wrap(err, "User.Login")
}

func (u *User) UpdateOpenAt(ctx context.Context) error {
	if u.ID == 0 {
		return errors.New("u.ID == 0")
	}

	// Update once a day at the most.
	if u.OpenAt != nil && u.OpenAt.After(ztime.Now().Add(-24*time.Hour)) {
		return nil
	}

	u.OpenAt = ztype.Ptr(ztime.Now())
	err := zdb.Exec(ctx, `update users set open_at = ? where user_id = ? and site_id = ?`,
		u.OpenAt, u.ID, MustGetSite(ctx).IDOrParent())
	return errors.Wrap(err, "User.UpdateOpenAt")
}

// Logout a user.
func (u *User) Logout(ctx context.Context) error {
	if u.ID == 0 {
		return errors.New("u.ID == 0")
	}

	u.LoginToken = nil
	u.LoginRequest = nil
	u.LoginAt = nil
	err := zdb.Exec(ctx,
		`update users set login_token=null, login_request=null where user_id=$1 and site_id=$2`,
		u.ID, MustGetSite(ctx).IDOrParent())
	return errors.Wrap(err, "User.Logout")
}

// CSRFToken gets the CSRF token.
func (u *User) CSRFToken() string {
	if u.Token == nil {
		return ""
	}
	return *u.Token
}

// HasAccess checks if this user has access to this site for the permission.
func (u User) HasAccess(check UserAccess) bool {
	switch check {
	default:
		return false
	case AccessSuperuser:
		return u.Access["all"] == AccessSuperuser
	case AccessAdmin:
		return u.Access["all"] == AccessSuperuser || u.Access["all"] == AccessAdmin
	case AccessSettings:
		return u.Access["all"] == AccessSuperuser || u.Access["all"] == AccessAdmin || u.Access["all"] == AccessSettings
	case AccessReadOnly:
		return u.Access["all"] == AccessSuperuser || u.Access["all"] == AccessAdmin || u.Access["all"] == AccessSettings || u.Access["all"] == AccessReadOnly
	}
}

func (u User) AccessSuperuser() bool { return u.Access["all"] == AccessSuperuser }
func (u User) AccessAdmin() bool     { return u.AccessSuperuser() || u.Access["all"] == AccessAdmin }
func (u User) AccessSettings() bool  { return u.AccessAdmin() || u.Access["all"] == AccessSettings }

// EmailReportRange gets the time range of the next report to send out.
//
// user.LastReportAt is set when a report is sent; to get the range for the new
// report we take LastReportAt, go to the start and end of the period, and if
// the endDate > now then send out a new report and set LastReportAt.
//
// The cronjob will send the report if the current date is after the end date.
func (u User) EmailReportRange() ztime.Range {
	var (
		start, end ztime.Time
		lastReport = ztime.Time{u.LastReportAt.In(u.Settings.Timezone.Loc())}
		week       = ztime.Week(u.Settings.SundayStartsWeek)
	)
	switch u.Settings.EmailReports.Int() {
	case EmailReportNever:
		return ztime.Range{}

	case EmailReportDaily:
		start, end = lastReport.StartOf(ztime.Day), lastReport.EndOf(ztime.Day)

	case EmailReportBiWeekly:
		start, end = lastReport.StartOf(week), lastReport.EndOf(week).AddPeriod(1, week)

	case EmailReportMonthly:
		start, end = lastReport.StartOf(ztime.Month), lastReport.EndOf(ztime.Month)

	case EmailReportWeekly:
		start, end = lastReport.StartOf(week), lastReport.EndOf(week)
	default:
		zlog.Errorf("invalid EmailReports value for user %d: %d", u.ID, u.Settings.EmailReports.Int())
		return ztime.Range{}
	}

	return ztime.NewRange(start.Time.Truncate(time.Second)).To(end.Time.Truncate(time.Second))
}

func (u User) EmailShort() string {
	local, _, ok := strings.Cut(u.Email, "@")
	if ok {
		return local + "@"
	}
	return u.Email
}

type Users []User

// List all users for a site.
func (u *Users) List(ctx context.Context, siteID int64) error {
	var s Site
	err := s.ByID(ctx, siteID)
	if err != nil {
		return err
	}
	return errors.Wrap(zdb.Select(ctx, u,
		`select * from users where site_id=$1`, s.IDOrParent()), "Users.List")
}

// Admins returns just the admins and superusers in this user list.
func (u Users) Admins() Users {
	n := make(Users, 0, len(u))
	for _, uu := range u {
		if uu.AccessSuperuser() || uu.AccessAdmin() {
			n = append(n, uu)
		}
	}
	return n
}

// ByEmail gets all users with this email address.
func (u *Users) ByEmail(ctx context.Context, email string) error {
	err := zdb.Select(ctx, u,
		`select * from users where lower(email)=lower($1) order by user_id asc`, email)
	return errors.Wrap(err, "Users.ByEmail")
}

// BySite gets all users for a site.
func (u *Users) BySite(ctx context.Context, siteID int64) error {
	err := zdb.Select(ctx, u,
		`select * from users where site_id=? order by user_id asc`, siteID)
	return errors.Wrap(err, "Users.BySite")
}

// Find users: by ID if ident is a number, or by email if it's not.
func (u *Users) Find(ctx context.Context, ident []string) error {
	ids, strs := splitIntStr(ident)
	err := zdb.Select(ctx, u, `select * from users where
		{{:ids user_id in (:ids) or}}
		{{:strs! 0=1}}
		{{:strs email in (:strs)}}`,
		map[string]any{"ids": ids, "strs": strs})
	return errors.Wrap(err, "Users.Find")
}

// IDs gets a list of all IDs for these users.
func (u *Users) IDs() []int64 {
	ids := make([]int64, 0, len(*u))
	for _, uu := range *u {
		ids = append(ids, uu.ID)
	}
	return ids
}

type (
	UserAccesses map[string]UserAccess
	UserAccess   string
)

const (
	AccessReadOnly  UserAccess = "r"
	AccessSettings  UserAccess = "s"
	AccessAdmin     UserAccess = "a"
	AccessSuperuser UserAccess = "*"
)

// TODO: this is not translated.
func (u UserAccess) String() string {
	switch u {
	case AccessReadOnly:
		return "read only"
	case AccessSettings:
		return "settings"
	case AccessAdmin:
		return "admin"
	case AccessSuperuser:
		return "superuser"
	default:
		panic(fmt.Sprintf("UserAccess is %q; should never happpen", string(u)))
	}
}

// Value implements the SQL Value function to determine what to store in the DB.
func (u UserAccesses) Value() (driver.Value, error) { return json.Marshal(u) }

// Scan converts the data returned from the DB into the struct.
func (u *UserAccesses) Scan(v any) error {
	switch vv := v.(type) {
	case []byte:
		return json.Unmarshal(vv, u)
	case string:
		return json.Unmarshal([]byte(vv), u)
	default:
		return fmt.Errorf("UserAccesses.Scan: unsupported type: %T", v)
	}
}

// Delete all users in this selection.
func (u *Users) Delete(ctx context.Context, force bool) error {
	err := zdb.TX(ctx, func(ctx context.Context) error {
		for _, uu := range *u {
			err := uu.Delete(WithSite(ctx, &Site{ID: uu.Site}), force)
			if err != nil {
				return err
			}
		}
		return nil
	})
	return errors.Wrap(err, "Users.Delete")
}

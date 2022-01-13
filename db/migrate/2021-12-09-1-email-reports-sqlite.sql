create table users2 (
	user_id        integer        primary key autoincrement,
	site_id        integer        not null,

	email          varchar        not null                 check(length(email) > 5 and length(email) <= 255),
	email_verified integer        not null default 0,
	password       blob           default null,
	totp_enabled   integer        not null default 0,
	totp_secret    blob,
	access         varchar      not null default '{"all":"a"}',
	login_at       timestamp      null                     check(login_at = strftime('%Y-%m-%d %H:%M:%S', login_at)),
	login_request  varchar        null,
	login_token    varchar        null,
	csrf_token     varchar        null,
	email_token    varchar        null,
	seen_updates_at timestamp     not null default current_timestamp check(seen_updates_at = strftime('%Y-%m-%d %H:%M:%S', seen_updates_at)),
	reset_at       timestamp      null,
	settings       varchar        not null default '{}',
	last_report_at timestamp      not null default current_timestamp,

	created_at     timestamp      not null                 check(created_at = strftime('%Y-%m-%d %H:%M:%S', created_at)),
	updated_at     timestamp                               check(updated_at = strftime('%Y-%m-%d %H:%M:%S', updated_at)),

	foreign key (site_id) references sites(site_id) on delete restrict on update restrict
);
insert into users2
	select user_id, site_id, email, email_verified, password, totp_enabled,
		totp_secret, access, login_at, login_request, login_token, csrf_token,
		email_token, seen_updates_at, reset_at, settings, datetime(), created_at,
		updated_at
	from users;

drop table users;
alter table users2 rename to users;
create        index "users#site_id"       on users(site_id);
create unique index "users#site_id#email" on users(site_id, lower(email));

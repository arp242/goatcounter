begin;
	--alter table users drop constraint users_name_check;
	create table users2 (
		id             integer        primary key autoincrement,
		site           integer        not null                 check(site > 0),

		name           varchar        not null,
		email          varchar        not null                 check(length(email) > 5 and length(email) <= 255),
		role           varchar        not null default ''      check(role in ('', 'a')),
		login_at       timestamp      null                     check(login_at = strftime('%Y-%m-%d %H:%M:%S', login_at)),
		login_request  varchar        null,
		login_token    varchar        null,
		csrf_token     varchar        null,
		seen_updates_at timestamp     not null default '1970-01-01 00:00:00' check(seen_updates_at = strftime('%Y-%m-%d %H:%M:%S', seen_updates_at)),

		created_at     timestamp      not null                 check(created_at = strftime('%Y-%m-%d %H:%M:%S', created_at)),
		updated_at     timestamp                               check(updated_at = strftime('%Y-%m-%d %H:%M:%S', updated_at)),

		foreign key (site) references sites(id) on delete restrict on update restrict
	);

	insert into users2 select * from users;
	drop table users;
	alter table users2 rename to users;

	alter table users add column password blob default null;
	alter table users add column email_verified int not null default 0;
	alter table users add column email_token varchar null;
	update users set email_verified=1;

	alter table users add column reset_at timestamp null;

	insert into version values ('2020-04-16-1-pwauth');
commit;

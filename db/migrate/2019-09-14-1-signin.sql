begin;
	create table user_keys (
		site           integer        not null                 check(site > 0),
		"user"         integer        not null                 check("user" > 0),

		login_req      timestamp      null,
		login_key      varchar        null,
		csrf_token     varchar        null,

		foreign key (site)   references sites(id) on delete restrict on update restrict,
		foreign key ("user") references users(id) on delete restrict on update restrict
	);
	create unique index "user_keys#login_key" on user_keys(login_key);

	insert into user_keys (select site, id, login_req, login_key, csrf_token from users);

	drop index "users#login_key";
	alter table users drop column login_req;
	alter table users drop column login_key;
	alter table users drop column csrf_token;
commit;

begin;
	create table api_tokens (
		api_token_id   integer        primary key autoincrement,
		site_id        integer        not null,
		user_id        integer        not null,
		name           varchar        not null,
		token          varchar        not null    check(length(token) > 10),
		permissions    jsonb          not null,
		created_at     timestamp      not null    check(created_at = strftime('%Y-%m-%d %H:%M:%S', created_at)),

		foreign key (site_id) references sites(id) on delete restrict on update restrict,
		foreign key (user_id) references users(id) on delete restrict on update restrict
	);
	create unique index "api_tokens#site_id#token" on api_tokens(site_id, token);

	insert into version values('2020-06-26-1-api-tokens');
commit;

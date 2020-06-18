begin;
	alter table users
		add column totp_enabled integer not null default 0,
		add column totp_secret bytea not null;

	insert into version values('2020-06-18-1-totp');
commit;

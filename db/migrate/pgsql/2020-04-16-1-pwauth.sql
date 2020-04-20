begin;
	alter table users add column password bytea default null;
	alter table users add column email_verified int not null default 0;
	alter table users add column email_token varchar null;
	update users set email_verified=1;

	alter table users drop constraint users_name_check;

	alter table users add column reset_at timestamp null;

	insert into version values ('2020-04-16-1-pwauth');
commit;

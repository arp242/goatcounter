begin;
	alter table users rename column login_key to login_request;
	alter table users rename column login_req to login_at;
	alter table users add    column login_token varchar null;

	drop index "users#login_key";
	create unique index "users#login_request" on users(login_request);
	create unique index "users#login_token"   on users(login_token);

	update users set login_token=login_request, login_request=null;

	insert into version values ('2019-09-16-2-users-token');
commit;


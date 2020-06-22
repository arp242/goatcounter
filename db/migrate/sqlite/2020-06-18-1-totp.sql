begin;
	alter table users add column totp_enabled integer not null default 0;
	alter table users add column totp_secret blob;

	update users set totp_secret=randomblob(16);

	insert into version values('2020-06-18-1-totp');
commit;

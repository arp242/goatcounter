begin;
	alter table hits
		add column location varchar not null default '';

	insert into version values ('2019-10-16-1-geoip');
commit;

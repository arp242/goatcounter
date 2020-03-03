begin;
	create table flags (
		name  varchar not null,
		value int     not null
	);

	insert into version values ('2020-03-03-1-flag');
commit;

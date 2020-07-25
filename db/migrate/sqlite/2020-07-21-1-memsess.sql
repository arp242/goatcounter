begin;
	create table store (
		key     varchar,
		value   text
	);
	create unique index "store#key" on store(key);

	insert into version values('2020-07-21-1-memsess');
commit;

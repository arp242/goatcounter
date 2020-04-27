begin;
	drop table usage;
	drop table flags;

	insert into version values ('2020-04-27-1-usage-flags');
commit;

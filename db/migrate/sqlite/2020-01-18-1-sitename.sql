begin;
	drop index "sites#name";
	insert into version values ('2020-01-18-1-sitename');
commit;

begin;
	alter table sites add column
		cname varchar null check(cname is null or (length(cname) >= 4 and length(cname) <= 255));
	create unique index "sites#cname" on sites(lower(cname));

	insert into version values ('2019-10-08-1-cname');
commit;

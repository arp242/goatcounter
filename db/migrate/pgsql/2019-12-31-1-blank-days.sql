begin;
	delete from hit_stats where stats='[0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0]';
	insert into version values ('2019-12-31-1-blank-days');
commit;

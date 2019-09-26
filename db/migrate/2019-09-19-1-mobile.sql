begin;
	alter table browser_stats
		add column mobile boolean default false not null;

	update sites set last_stat=null;
	delete from hit_stats;
	delete from browser_stats;

	insert into version values ('2019-09-19-1-mobile');
commit;

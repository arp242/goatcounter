begin;
	alter table hit_stats drop column total;
	alter table sites     drop column last_stat;

	alter table sites
		drop constraint if exists sites_domain_check;
	alter table sites
		add constraint sites_link_domain_check check(link_domain = '' or (length(link_domain) >= 4 and length(link_domain) <= 255));

	insert into version values ('2020-03-16-2-rm-old');
commit;

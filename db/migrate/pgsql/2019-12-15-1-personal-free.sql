begin;
	update sites set plan = 'personal' where plan = 'personal-free';

	alter table sites
		drop constraint sites_plan_check;
	alter table sites
		add constraint sites_plan_check check(plan in ('personal', 'starter', 'pro', 'child', 'custom'));

	insert into version values ('2019-12-15-1-personal-free');
commit;

begin;
	alter table sites
		drop constraint sites_plan_check;

	update sites set plan = 'personal-free' where plan = 'p';
	update sites set plan = 'pro' where plan = 'b';
	update sites set plan = 'child' where plan = 'c';

	alter table sites
		add constraint sites_plan_check check(plan in ('personal-free', 'personal', 'starter', 'pro', 'child', 'custom'));

	insert into version values ('2019-12-10-1-plans');
commit;

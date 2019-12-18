begin;
	alter table sites
		drop constraint sites_plan_check;
	update sites set plan = 'business' where plan = 'starter';
	update sites set plan = 'businessplus' where plan = 'pro';
	alter table sites
		add constraint sites_plan_check check(plan in ('personal', 'business', 'businessplus', 'child', 'custom'));

	insert into version values ('2019-12-17-1-business');
commit;

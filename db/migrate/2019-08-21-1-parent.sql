begin;
	-- Add parent column
	alter table sites add column parent integer null check(parent is null or parent > 0);

	-- Allow 'c' plan.
	alter table sites drop constraint sites_plan_check;
	alter table sites add  constraint sites_plan_check check(plan in ('p', 'b', 'e', 'c'));

	-- Rename sites.domain to sites.name
	alter table sites rename domain to name;
	alter index "sites#domain" rename to "sites#name";

	-- Update my sites
	-- delete from users where site in (2, 3, 4, 5, 7);
	-- update sites set parent=1, plan='c' where id in (2, 3, 4, 5, 7);

	-- Merge vimlog in to arp242
	-- update hits      set site=1 where site=7;
	-- update hit_stats set site=1 where site=7;
	-- update browsers  set site=1 where site=7;
	-- delete from sites where id=7;

	insert into version values ('2019-08-21-1-parent');
commit;

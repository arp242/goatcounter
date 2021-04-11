alter table sites add column if not exists billing_anchor timestamp;
alter table sites add column if not exists notes text not null default '';
alter table sites add column if not exists extra_pageviews int;
alter table sites add column if not exists extra_pageviews_sub varchar;

alter table sites drop constraint sites_plan_check;

update sites set plan='starter'           where plan='personalplus';
update sites set plan='trial'             where stripe is null;
update sites set plan='free', stripe=null where stripe like 'cus_free_%';
update sites set plan='child'             where parent is not null;

update sites
	set settings = jsonb_set(to_jsonb(settings), '{data_retention}', '31', true)
where
	cast(settings->'data_retention' as int) != 0 and
	cast(settings->'data_retention' as int) < 31;

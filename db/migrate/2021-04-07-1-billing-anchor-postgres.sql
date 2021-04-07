alter table sites add column billing_anchor timestamp;
alter table sites add column notes text not null default '';
alter table sites add column extra_pageviews int;

alter table sites drop constraint sites_plan_check;

update sites set plan='starter'           where plan='personalplus';
update sites set plan='trial'             where stripe is null;
update sites set plan='free', stripe=null where stripe like 'cus_free_%';

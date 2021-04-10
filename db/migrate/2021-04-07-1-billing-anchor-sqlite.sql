alter table sites add column billing_anchor timestamp;
alter table sites add column notes text not null default '';
alter table sites add column extra_pageviews int;
alter table sites add column extra_pageviews_sub varchar;

-- No need to update sites as SQLite doesn't support saas/billing.
alter table sites drop column plan;
alter table sites add column plan varchar not null default 'free';

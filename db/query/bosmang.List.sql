with
	accounts as (
		select
			site_id as site_id,
			(select a.site_id || array_agg(site_id)      from sites c where c.parent = a.site_id) as allsites,
			(select string_agg(code, ' | ') from sites d where d.site_id = a.site_id or d.parent = a.site_id) as codes
		from sites a
		where parent is null
		group by site_id
		order by site_id asc
	),
	total as (
		select site_id, sum(total) as t from hit_counts group by site_id
	),
	last_month as (
		select site_id, sum(total) as t from hit_counts where hour >= now() - interval '30 days' group by site_id
	),
	grouped as (
		select
			accounts.site_id,
			(select coalesce(sum(t), 0) from total      where total.site_id      = any(accounts.allsites)) as total,
			(select coalesce(sum(t), 0) from last_month where last_month.site_id = any(accounts.allsites)) as last_month,
			codes
		from accounts
		group by accounts.site_id, codes, allsites
		order by last_month desc
	)
select
	grouped.site_id,
	grouped.total,
	created_at,
	grouped.last_month,
	(coalesce(total, 0) / greatest(extract('days' from now() - created_at), 1) * 30.5)::int as avg,
	grouped.codes
from grouped
join sites using (site_id)
where last_month > 10000 or total > 500000

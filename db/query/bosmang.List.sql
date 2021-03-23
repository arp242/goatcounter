with
	x as (
		select site_id, sum(total) as t from hit_counts group by site_id
	),
	y as (
		select site_id, sum(total) as t from hit_counts where hour >= now() - interval '30 days' group by site_id
	)
select
	site_id,
	parent,
	code,
	created_at,
	billing_amount,
	(case
		when stripe is null then 'free'
		when substr(stripe, 0, 9) = 'cus_free' then 'free'
		else plan
	end) as plan,
	stripe,
	coalesce(x.t, 0) as total,
	coalesce(y.t, 0) as last_month
from sites
left join x using (site_id)
left join y using (site_id)
order by last_month desc

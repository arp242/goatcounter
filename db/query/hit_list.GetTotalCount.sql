with totals as (
	select
		coalesce(sum(total), 0)        as total,
		coalesce(sum(total_unique), 0) as total_unique
	from hit_counts
	where
		site_id = :site and hour >= :start and hour <= :end
		{{:filter and path_id in (:filter)}}
)
select
	*,

	-- Get the UTC offset for the browser, screen size, etc. charts, which are
	-- always stored in UTC.
	totals.total_unique - (
		select coalesce(sum(total_unique), 0)
		from hit_counts
		where site_id = :site and
			{{:filter path_id in (:filter) and}}

			-- PostgreSQL
			{{:sqlite! (hour >= :start and hour < cast(:start as timestamp) + :tz * interval '1 minute') or }}
			{{:sqlite! (hour >= :end   and hour < cast(:end as timestamp)   + :tz * interval '1 minute')    }}

			-- SQLite
			{{:sqlite (hour >= :start and hour < datetime(:start, :tz || 'minute')) or }}
			{{:sqlite (hour >= :end   and hour < datetime(:end,   :tz || 'minute'))    }}
	) as total_unique_utc
from totals;

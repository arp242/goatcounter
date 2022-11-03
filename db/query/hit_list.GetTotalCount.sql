with x as (
	select
		coalesce(sum(total_unique), 0) as total_unique
	from hit_counts
	where
		site_id = :site and hour >= :start and hour <= :end
		{{:filter and path_id in (:filter)}}
), y as (
	select
		coalesce(sum(total_unique), 0) as total_events_unique
	from hit_counts
	join paths using (site_id, path_id)
	where
		hit_counts.site_id = :site and hour >= :start and hour <= :end and paths.event = 1
		{{:filter and path_id in (:filter)}}
), z as (
	select
		coalesce(sum(total_unique), 0) as total_unique_utc
	from hit_counts
	where
		site_id = :site and hour >= :start_utc and hour <= :end_utc
		{{:filter and path_id in (:filter)}}
)
select
	*
	-- TODO the below should be faster, but isn't quite correct.
	--
	-- Get the UTC offset for the browser, screen size, etc. charts, which are
	-- always stored in UTC. Instead of calculating everything again, substract the
	-- pageviews outside of the UTC range, which is a lot faster.
	-- x.total_unique - (
	-- 	select coalesce(sum(total_unique), 0)
	-- 	from hit_counts
	-- 	where site_id = :site and
	-- 		{{:filter path_id in (:filter) and}}
	-- 		(hour >= :start and hour <= cast(:start as timestamp) + :tz * interval '1 minute') or
	-- 		(hour >= :end   and hour <= cast(:end   as timestamp) + :tz * interval '1 minute')
	-- ) as total_unique_utc
from x, y, z;

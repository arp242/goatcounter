-- TODO this reads like ~800k rows and 80M of data for some larger sites. That's
-- obviously not ideal.
--
-- Precomputing values (e.g. though a materialized view) is hard, as we need to
-- get everything local to the user's configured TZ, so we can't calculate daily
-- sums (which is a lot faster).
--
-- So, not sure what to do with this.
select coalesce(sum(total), 0) as t
from hit_counts
where
	site_id = :site and
	{{:filter path_id in (:filter) and}}
	hour >= :start and hour <= :end
{{:sqlite group by path_id, date(hour, :tz)}}
{{:pgsql  group by path_id, date(timezone(:tz, hour))}}
order by t desc
limit 1

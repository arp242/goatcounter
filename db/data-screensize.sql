select
	date(created_at) as date,
	count(*) as count,
	replace(size, ' ', '') as size
from hits
where
	size != '' and
	first_visit=1
group by date, size
order by date, size;

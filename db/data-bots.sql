select
    date(created_at) as date,
    count(*) as count,
    browser as user_agent
from hits
where
	created_at > '2019-07-29' and
	bot in (3, 4, 5, 6, 7, 150, 151, 152, 153) and
	first_visit=1
group by date, user_agent
order by date, user_agent;

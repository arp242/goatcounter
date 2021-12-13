with x as (
    select count(*) as count, site_id from users group by site_id
)
update users set access = '{"all": "*"}' from x
where x.count = 1 and users.site_id = x.site_id

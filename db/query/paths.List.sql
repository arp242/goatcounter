select * from paths
where
	site_id = :site
	{{:after and path_id > :after}}
order by path_id asc
{{:limit limit :limit}}

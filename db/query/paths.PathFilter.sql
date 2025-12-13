select path_id from paths
where
	site_id = :site
	{{:only_event    and event=1}}
	{{:only_pageview and event=0}}
	and (
		{{:match_path  lower(path)  :not like lower(:like)}}
		:or
		{{:match_title lower(title) :not like lower(:like)}}
	)

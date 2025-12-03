select path_id from paths
where
	site_id = :site
	{{:invert and not ( 1=1}}
		{{:only_event    and event=1}}
		{{:only_pageview and event=0}}
		{{:have_like and (}}
			{{:match_path  lower(path)  :not like lower(:like)}}
			:or
			{{:match_title lower(title) :not like lower(:like)}}
		{{:have_like )}}
	{{:invert )}}

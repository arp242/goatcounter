select path_id from paths
where
	site_id = :site
	{{:only_event    and event=1}}
	{{:only_pageview and event=0}}
	and (
		{{:match_path  lower(path)  :not like lower(:filter)}}
		:or
		{{:match_title lower(title) :not like lower(:filter)}}
	)

-- The limit is here because that's the limit in SQL parameters; the returned
-- []int64 is passed as parameters later on.
--
-- Having (and scrolling!) more than 65k pages is a rather curious usage
-- pattern, but it has happened. This was just because they were sending data
-- with many unique IDs in the URL which really ought to be removed.
limit 65500

select path_id from paths
where
	site_id = :site and (
		lower(path) like lower(:filter)
		{{:match_title or lower(title) like lower(:filter)}}
	)
-- The limit is here because that's the limit in SQL parameters; the returned
-- []int64 is passed as parameters later on.
--
-- Having (and scrolling!) more than 65k pages is a rather curious usage
-- pattern, but it has happened. This was just because they were sending data
-- with many unique IDs in the URL which really ought to be removed.
limit 65500

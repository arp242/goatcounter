begin;
	create index tmp on hits(browser);
	update hits set
		path_id=(select path_id from paths where paths.site_id=hits.site and paths.path=hits.path),
		user_agent_id=(select user_agent_id from user_agents where ua=hits.browser);
	drop index tmp;

	update hits set
		session2=cast(substr(session || '0000000000000000' , 1, 16) as blob)
		where session is not null;

	insert into version values('2020-08-28-2-paths-paths');
commit;

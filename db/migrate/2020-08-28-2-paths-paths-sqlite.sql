create index tmp1 on hits(browser);
create index tmp2 on paths(site_id, lower(path));

update hits set
	path_id=(select path_id from paths where paths.site_id=hits.site and lower(paths.path)=lower(hits.path)),
	user_agent_id=(select user_agent_id from user_agents where ua=hits.browser);

drop index tmp1;
drop index tmp2;

update hits set
	session2=cast(substr(session || '0000000000000000' , 1, 16) as blob)
	where session is not null;

alter table hits set unlogged;

create index tmp1 on hits(browser);
create index tmp2 on paths(site_id, lower(path));

update hits set
	path_id=(select path_id from paths where paths.site_id=hits.site and lower(paths.path)=lower(hits.path)),
	user_agent_id=(select user_agent_id from user_agents where ua=hits.browser);

drop index tmp1;
drop index tmp2;

update hits set
	session2=cast(rpad(cast(session as varchar), 16, '0') as bytea)
	where session is not null;

alter table hits set logged;

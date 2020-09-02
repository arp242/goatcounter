begin;
	create view view_user_agents as
		select
			user_agents.user_agent_id as id,
			user_agents.system_id     as bid,
			user_agents.browser_id    as sid,
			user_agents.bot,
			browsers.name || ' ' || browsers.version as browser,
			systems.name  || ' ' || systems.version as system,
			user_agents.ua
		from user_agents
		join browsers using (browser_id)
		join systems using (system_id);

	create view hits_export as
		select
			hits.hit_id,
			hits.site_id,

			paths.path,
			paths.title,
			paths.event,

			user_agents.ua,
			browsers.name || ' ' || browsers.version as browser,
			systems.name || ' ' || systems.version as system,

			hits.session,
			hits.bot,
			hits.ref,
			hits.ref_scheme as ref_s,
			hits.size,
			hits.location as loc,
			hits.first_visit as first,
			hits.created_at
		from hits
		join paths       using (site_id, path_id)
		join user_agents using (user_agent_id)
		join browsers    using (browser_id)
		join systems     using (system_id)
		order by hit_id asc;

	insert into version values ('2020-08-28-5-paths-views');
commit;

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

	insert into version values ('2020-08-28-6-paths-views');
commit;

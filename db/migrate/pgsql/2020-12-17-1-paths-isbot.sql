begin;
	drop view if exists view_user_agents;
	alter table user_agents rename column bot to isbot;

	insert into version values('2020-12-17-1-paths-isbot');
commit;

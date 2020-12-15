begin;
	alter table user_agents add foreign key (browser_id) references browsers(browser_id) on delete restrict on update restrict;
	alter table user_agents add foreign key (system_id)  references systems(system_id)   on delete restrict on update restrict;

	alter table hits alter column path_id drop default;
	alter table hits alter column user_agent_id drop default;

	insert into version values('2020-08-28-5-paths-ua-fk');
commit;

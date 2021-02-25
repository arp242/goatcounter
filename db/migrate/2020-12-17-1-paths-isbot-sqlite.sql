drop view if exists view_user_agents;
create table user_agents2 (
	user_agent_id  integer        primary key autoincrement,
	browser_id     integer        not null,
	system_id      integer        not null,

	ua             varchar        not null,
	isbot          integer        not null,

	foreign key (browser_id) references browsers(browser_id) on delete restrict on update restrict,
	foreign key (system_id)  references systems(system_id)   on delete restrict on update restrict
);
insert into user_agents2 select user_agent_id, browser_id, system_id, ua, bot from user_agents;
drop table user_agents;
alter table user_agents2 rename to user_agents;
create unique index "user_agents#ua" on user_agents(ua);

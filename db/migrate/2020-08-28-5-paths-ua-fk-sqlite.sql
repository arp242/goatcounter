create table user_agents2 (
	user_agent_id    integer        primary key autoincrement,
	browser_id       int            not null,
	system_id        int			not null,

	ua               varchar        not null,
	bot              int            not null,

	foreign key (browser_id) references browsers(browser_id) on delete restrict on update restrict,
	foreign key (system_id)  references systems(system_id)   on delete restrict on update restrict
);

insert into user_agents2
	select user_agent_id, browser_id, system_id, ua, bot from user_agents;
drop table user_agents;
alter table user_agents2 rename to user_agents;
create unique index "user_agents#ua" on user_agents(ua);

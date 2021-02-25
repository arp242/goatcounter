drop view if exists view_user_agents;
alter table user_agents rename column bot to isbot;

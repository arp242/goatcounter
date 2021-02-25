update sites set settings = jsonb_set(settings, '{collect}', '30', true);

alter table hits alter column user_agent_id drop not null;
alter table hits drop constraint hits_user_agent_id_check;
alter table hits add constraint hits_user_agent_id_check check(user_agent_id != 0);

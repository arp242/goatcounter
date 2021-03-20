alter table users add column access jsonb not null default '{"all":"a"}';

alter table exports add   column format text not null default 'csv';
alter table exports alter column format drop default;

alter table exports add   column start_from_day date;
alter table exports alter column start_from_hit_id drop not null;

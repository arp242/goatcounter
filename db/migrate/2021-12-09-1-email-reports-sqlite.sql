alter table users add column last_report_at timestamp not null default current_date;

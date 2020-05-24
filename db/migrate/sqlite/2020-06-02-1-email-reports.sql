begin;
	alter table users add column last_report_at timestamp default null;

	insert into version values ('2020-06-02-1-email-reports');
commit;

begin;
	update hits set started_session=1 where id in (
		select min(id) from hits where session>0 and started_session=1 group by path
	);
	alter table hits rename column started_session to first_visit;

	alter table sessions add column paths varchar[];

	insert into version values ('2020-05-13-1-unique-path');
commit;

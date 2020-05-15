begin;
	update hits set started_session=1 where id in
		(select min(id) from hits where session>0 and started_session=0 group by path, session);
	alter table hits rename column started_session to first_visit;

	create table session_paths (
		session integer not null,
		path    varchar not null,

		foreign key (session) references sessions(id) on delete cascade on update cascade
	);

	insert into version values ('2020-05-13-1-unique-path');
commit;

begin;
	alter table hits drop constraint hits_ref_scheme_check;
	alter table hits add  constraint hits_ref_scheme_check check(ref_scheme in ('h', 'g', 'o'));

	update hits set ref_original=ref, ref='Hacker News', ref_scheme='g' where ref in (
		'io.github.hidroh.materialistic',
		'hackerweb.app',
		'www.daemonology.net/hn-daily',
		'quiethn.com'
	);

	insert into version values ('2019-08-19-1-ref_schema');
commit;

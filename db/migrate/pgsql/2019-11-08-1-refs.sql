begin;
	update hits
		set ref_original=ref, ref='Hacker News', ref_scheme='g'
		where ref in ('hnews.xyz', 'hackernewsmobile.com');

	update hits
		set ref_original=ref, ref='RSS', ref_scheme='g'
		where ref in ('org.fox.ttrss', 'www.inoreader.com', 'com.innologica.inoreader');

	update hits
		set ref='RSS', ref_scheme='g'
		where ref like 'feedly.com%';

	update hits
		set ref_original=ref, ref='RSS', ref_scheme='g'
		where ref like 'usepanda.com%';

	insert into version values ('2019-11-08-1-refs');
commit;

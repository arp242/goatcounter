delete from store where key='session';

insert into store (key, value) values ('display-once',
	'If you just upgraded to 2.0 then you need to run "goatcounter reindex" to
rebuild some tables; see the release notes:
https://github.com/arp242/goatcounter/releases/tag/v2.0.0

There are also some other incompatible changes.');

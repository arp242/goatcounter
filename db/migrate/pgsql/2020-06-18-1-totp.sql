begin;
	alter table users
		add column totp_enabled integer not null default 0,
		add column totp_secret bytea;

	-- https://dba.stackexchange.com/a/22571/2629
	CREATE OR REPLACE FUNCTION random_bytea(bytea_length integer)
	RETURNS bytea AS $body$
		SELECT decode(string_agg(lpad(to_hex(width_bucket(random(), 0, 1, 256)-1),2,'0') ,''), 'hex')
		FROM generate_series(1, $1);
	$body$
	LANGUAGE 'sql'
	VOLATILE
	SET search_path = 'pg_catalog';

	update users set totp_secret=random_bytea(16);

	drop function random_bytea;

	insert into version values('2020-06-18-1-totp');
commit;

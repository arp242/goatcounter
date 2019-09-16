begin;
	alter table hits add column
		browser        varchar        not null default '';

	-- Every browser row should have a hit with the same created date. This
	-- won't be the same path, but that's okay for now.
	create function mig() returns void as $$
	declare
		rec record;
	begin
		for rec in select * from browsers loop
			update hits set browser=rec.browser
				where site=rec.site and created_at=rec.created_at and
				CTID in (select CTID from hits where site=rec.site and browser='' and created_at=rec.created_at limit 1);
		end loop;
	end;
	$$ language plpgsql;
	select mig();

	-- Remove old size, since it's not matched with a browser.
	alter table hits alter column size set default '';
	update hits set size='';

	drop table browsers;
	insert into version values ('2019-09-16-1-hits-browsers');
commit;

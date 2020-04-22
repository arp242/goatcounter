begin;
	alter table hits drop constraint hits_ref_scheme_check;
	alter table hits add constraint hits_ref_scheme_check check(ref_scheme in ('h', 'g', 'o', 'c'));

	insert into version values ('2020-04-22-1-campaigns');
commit;

begin;
	alter table sites alter column settings type json using settings::json;
	insert into version values ('2020-03-18-1-json_settings');
commit;

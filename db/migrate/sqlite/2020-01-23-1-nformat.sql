begin;
	update sites set settings=substr(settings, 0, length(settings)) || ', "number_format": 8239}';
	insert into version values ('2020-01-23-1-nformat');
commit;

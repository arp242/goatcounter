alter table hits add column width smallint null;
update hits set
	width = (select width from sizes where size_id = hits.size_id);

alter table hits drop column size_id;
drop table sizes;

alter table sites add column user_defaults varchar not null default '{}';
update sites set user_defaults = (
    select json_object(
		'date_format',        json_extract(settings, '$.date_format'),
		'number_format',      json_extract(settings, '$.number_format'),
		'timezone',           json_extract(settings, '$.timezone'),
		'twenty_four_hours',  json_extract(settings, '$.twenty_four_hours'),
		'sunday_starts_week', json_extract(settings, '$.sunday_starts_week'),
		'views',              json_extract(settings, '$.views'),
		'widgets',            json_extract(settings, '$.widgets'))
    from sites s2 where s2.site_id = sites.site_id
);

-- SQLite converts booleans to 0/1; which we can't really deal with in the
-- application, and I'd prefer not to have to modify that.
update sites set user_defaults = replace(replace(replace(replace(user_defaults,
	'"twenty_four_hours":0', '"twenty_four_hours":false'),
	'"twenty_four_hours":1', '"twenty_four_hours":true'),
	'"sunday_starts_week":0', '"sunday_starts_week":false'),
	'"sunday_starts_week":1', '"sunday_starts_week":true');

update sites set settings = json_remove(settings, '$.date_format',
	'$.number_format', '$.timezone', '$.twenty_four_hours',
	'$.sunday_starts_week', '$.views', '$.widgets');

alter table users add column settings varchar not null default '{}';
update users set settings = (select user_defaults from sites where sites.site_id = users.site_id);

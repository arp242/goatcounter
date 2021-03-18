alter table sites add column user_defaults jsonb not null default '{}';
update sites set user_defaults = (
    select
        jsonb_set(jsonb_set(jsonb_set(jsonb_set(jsonb_set(jsonb_set(jsonb_set('{}',
            '{date_format}',        settings->'date_format'),
            '{number_format}',      settings->'number_format'),
            '{timezone}',           settings->'timezone'),
            '{twenty_four_hours}',  settings->'twenty_four_hours'),
            '{sunday_starts_week}', settings->'sunday_starts_week'),
            '{views}',              settings->'views'),
            '{widgets}',            settings->'widgets')
    from sites s2 where s2.site_id = sites.site_id
);
update sites set settings = settings - 'date_format' - 'number_format' -
	'timezone' - 'twenty_four_hours' - 'sunday_starts_week' - 'views' - 'widgets';

alter table users add column settings jsonb not null default '{}';
update users set settings = (select user_defaults from sites where sites.site_id = users.site_id);

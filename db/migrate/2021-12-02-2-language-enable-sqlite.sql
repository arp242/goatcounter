update sites set
	settings      = json_replace(settings, '$.collect', json_extract(settings, '$.collect') | 64),
	user_defaults = json_replace(user_defaults, '$.widgets', json_array(json_extract(user_defaults, '$.widgets'), json('{"n":"languages"}')));
update users set
	settings = json_replace(settings, '$.widgets', json_array(json_extract(settings, '$.widgets'), json('{"n":"languages"}')));

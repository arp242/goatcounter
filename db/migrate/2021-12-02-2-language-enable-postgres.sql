update sites set
	settings      = jsonb_set(settings, '{collect}', to_jsonb(cast(settings->'collect' as int) | 64)),
	user_defaults = jsonb_set(user_defaults, '{widgets}', user_defaults->'widgets' || '[{"n":"languages"}]');
update users set
	settings = jsonb_set(settings, '{widgets}', settings->'widgets' || '[{"n":"languages"}]');

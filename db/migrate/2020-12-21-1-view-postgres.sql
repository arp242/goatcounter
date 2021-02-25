update sites set settings = jsonb_set(settings, '{views}',
	'[{"name": "default", "period": "week"}]', true);

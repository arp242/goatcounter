update sites set settings = json_set(settings, '$.views',
	json('[{"name": "default", "period": "week"}]'));

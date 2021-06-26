update sites set settings = jsonb_set(settings, '{public}', '"public"')  where settings->'public' = 'true';
update sites set settings = jsonb_set(settings, '{public}', '"private"') where settings->'public' = 'false';

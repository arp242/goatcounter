update sites set settings = json_set(settings, '$.public', 'public')  where json_extract(settings, '$.public') = 1;
update sites set settings = json_set(settings, '$.public', 'private') where json_extract(settings, '$.public') = 0;

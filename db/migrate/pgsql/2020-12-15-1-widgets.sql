begin;
	alter table sites alter column settings type jsonb;
	update sites set settings = jsonb_set(settings, '{widgets}', '[
		{"name": "pages",      "on": true, "s": {"limit_pages": 10, "limit_refs": 10}},
		{"name": "totalpages", "on": true},
		{"name": "toprefs",    "on": true},
		{"name": "browsers",   "on": true},
		{"name": "systems",    "on": true},
		{"name": "sizes",      "on": true},
		{"name": "locations",  "on": true}]', true);
	update sites set settings = jsonb_set(settings, '{widgets,0,s,limit_pages}', settings->'limits'->'page');
	update sites set settings = jsonb_set(settings, '{widgets,0,s,limit_refs}',  settings->'limits'->'ref');
	update sites set settings = settings#-'{limits}';

	insert into version values('2020-12-15-1-widgets');
commit;

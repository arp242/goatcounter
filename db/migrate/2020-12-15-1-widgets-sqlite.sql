update sites set settings = json_set(settings, '$.widgets', json('[
	{"name": "pages",      "on": true, "s": {"limit_pages": 10, "limit_refs": 10}},
	{"name": "totalpages", "on": true},
	{"name": "toprefs",    "on": true},
	{"name": "browsers",   "on": true},
	{"name": "systems",    "on": true},
	{"name": "sizes",      "on": true},
	{"name": "locations",  "on": true}]'));
update sites set settings = json_replace(settings, '$.widgets[0].s.limit_pages', json_extract(settings, '$.limits.page'));
update sites set settings = json_replace(settings, '$.widgets[0].s.limit_refs',  json_extract(settings, '$.limits.ref'));
update sites set settings = json_remove(settings, '$.limits');

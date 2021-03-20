update api_tokens set permissions = 0 |
	iif(json_extract(permissions, '$.count'),        2, 0) |
	iif(json_extract(permissions, '$.export'),       4, 0) |
	iif(json_extract(permissions, '$.site_read'),    8, 0) |
	iif(json_extract(permissions, '$.site_create'), 16, 0) |
	iif(json_extract(permissions, '$.site_update'), 32, 0);

update api_tokens set permissions = to_jsonb(0 |
	case when cast(permissions->'count'       as bool) then  2 else 0 end |
	case when cast(permissions->'export'      as bool) then  4 else 0 end |
	case when cast(permissions->'site_read'   as bool) then  8 else 0 end |
	case when cast(permissions->'site_create' as bool) then 16 else 0 end |
	case when cast(permissions->'site_update' as bool) then 32 else 0 end);

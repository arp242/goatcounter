-- Add CollectSession bitmask
update sites set settings = jsonb_set(settings, '{collect}',
	to_jsonb(128 | cast(settings->'collect' as int)),
	true);

-- New accounts have it set to "US, CH, RU", but keep collecting everything for
-- existing sites.
update sites set settings = jsonb_set(settings, '{collect_regions}', '""', true);

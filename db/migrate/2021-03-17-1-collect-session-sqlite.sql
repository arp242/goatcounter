-- Add CollectSession bitmask
update sites set settings = json_set(settings, '$.collect', 128 | json_extract(settings, '$.collect'));

-- New accounts have it set to "US, CH, RU", but keep collecting everything for
-- existing sites.
update sites set settings = json_set(settings, '$.collect_regions', '');

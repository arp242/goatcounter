-- 20190611; split out ref path and parameters
alter table hits
    add column ref_params;
update hits set
	ref_params = case instr(ref, "?")
	             when 0 then null
	             else substr(ref, instr(ref, "?") + 1)
	             end,
	ref        = case instr(ref, "?")
	             when 0 then ref
	             else substr(ref, 0, instr(ref, "?"))
	             end;


-- 20190611; leading / for paths sent from the client wasn't normalized.
update hits set path = "/" || ltrim(path, "/");

-- 20190611; remove hits from local dev.
delete from hits where ref like "%localhost%";

-- 20190615: remove trailing slash from refs, paths
update hits set
	path = case path
	       when "/" then "/"
	       else rtrim(path, "/")
	       end,
	ref =  rtrim(ref, "/");

-- 20190625: add hit_stats table.
create table hit_stats (
	site           integer        not null check(site > 0),

	kind           varchar        not null check(kind in ("h", "d")), -- hourly, daily
	day            date           not null,  -- "2019-06-22"
	path           varchar        not null,  -- /foo.html
	stats          varchar        not null,  -- hourly or daily hits [20, 30, ...]

	created_at     datetime       null default current_timestamp,
	updated_at     datetime
);

-- 20190629: clean up ref_params
update hits set ref_params = null where ref_params = "";
update hits set ref_params = null where ref like "https://mail.google.com/%";
update hits set ref_params = null where ref_params = "amp=1";
update hits set ref_params = null where ref_params like "%utm_source=%"; -- Not 100% accurate as it removes everything, but that's okay.
update hits set ref_params = null where ref_params like "%utm_medium=%";
update hits set ref_params = null where ref_params like "%utm_term=%";
update hits set ref_params = null where ref_params like "%utm_campaign=%";

-- 20190629: add ref_original
alter table hits add column ref_original varchar;

-- update hits set
-- 	ref = case ref_params
-- 	      when null then ref
-- 	      when "" then ref
-- 	      else ref || "?" || ref_params
-- 	      end;

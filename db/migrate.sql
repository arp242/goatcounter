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

-- 20190705: group URLs
update hits set ref_original=ref, ref="Google" where
	ref like "https://www.google.%" or
	ref like "http://www.google.%" or ref in (
	"android-app://com.google.android.googlequicksearchbox",
	"android-app://com.google.android.googlequicksearchbox/https/www.google.com");

update hits set ref_original=ref, ref="Hacker News" where ref in (
	"https://news.ycombinator.com",
	"https://hn.algolia.com",
	"https://hckrnews.com",
	"https://hn.premii.com",
	"android-app://com.stefandekanski.hackernews.free");

update hits set ref_original=ref, ref="Gmail" where
	ref like "https://mail.google.com%" or
	ref in (
	"android-app://com.google.android.gm");

update hits set ref_original=ref, ref="https://www.reddit.com" where ref in (
	"android-app://com.andrewshu.android.reddit",
	"android-app://com.laurencedawson.reddit_sync",
	"android-app://com.laurencedawson.reddit_sync.dev",
	"android-app://com.laurencedawson.reddit_sync.pro",
	"https://old.reddit.com");

update hits set ref_original=ref, ref="https://link.oreilly.com" where ref like "https://link.oreilly.com%";
update hits set ref_original=ref, ref="https://link.oreilly.com" where ref like "http://link.oreilly.com%";

update hits set ref_original=ref, ref=replace(ref, "https://old.reddit.com", "https://www.reddit.com")
where ref like "https://old.reddit.com%";

update hits set ref_original=ref, ref=replace(ref, "https://en.m.wikipedia.org", "https://en.wikipedia.org")
where ref like "https://en.m.wikipedia.org%";

update hits set ref_original=ref, ref=replace(ref, "https://m.habr.com", "https://habr.com")
where ref like "https://m.habr.com%";

update hits set ref_original=ref, ref="https://www.facebook.com" where ref in (
	"android-app://m.facebook.com",
	"https://m.facebook.com",
	"http://m.facebook.com",
	"https://l.facebook.com",
	"https://lm.facebook.com");

update hits set ref_original=ref, ref="Telegram Messenger" where ref in (
	"android-app://org.telegram.messenger");

update hits set ref_original=ref, ref="Slack Chat" where ref in (
	"android-app://com.Slack");

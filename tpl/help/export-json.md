The JSON export is a ZIP file containing several JSON documents, documented
below.

If you fill in *Export from date* then it will export only pageviews from that
date.

**Warning:** statistics are stored with a resolution of an hour or day. If you
create an export at 7:30 on March 10 and then another one at 7:30 on March 20
with the start date set to March 10, you will have some duplicate statistics for
March 10 because we can't know which ones were already exported. To deal with
this:

- If you're importing a full set of pageviews from several incremental exports,
  then the last day should be ignored for all except the last export.

- If you want to keep a local database synced with GoatCounter, then you should
  overwrite the statistics for the same day.

Versions and compatibility
--------------------------
There is currently one export version: `1.0`.

Any compatible change will increase the minor version, any incompatible change
will increase the major version. Adding new JSON files or adding new fields to
existing files would be *compatible* changes. Changing or removing fields would
be *incompatible* changes.

Files
-----
`.json` files are complete JSON documents, with an object or array as the top
level, and may span multiple lines.

`.jsonl` files are in the "JSON lines" format. That is, every line contains a
JSON object:

    {"key": "value"}
    {"key": "value"}
    {"key": "value"}

### Data files

#### `info.json`
JSON object with information about the export:

    {
        "export_version":      "1.0",
        "goatcounter_version": "2.7",
        "created_for":         "stats.arp242.net",
        "created_by":          "user_id=1 <martin@arp242.net>",
        "created_at":          "2026-03-07T16:49:27Z"
    }

The `goatcounter_version` is identical to the output of `goatcounter version`,
and may be as `commit_date` if it's not a tagged release:
`7b4729b1bd99_2026-03-20T16:33:14Z`.

#### `languages.jsonl`
List of all languages, `iso_639_3` is referenced by `language_stats.language`.

    {"iso_639_3": "",    "name": "(unknown)"}
    {"iso_639_3": "aaa", "name": "Ghotuo"}
    {"iso_639_3": "aao", "name": "Algerian Saharan Arabic"}

#### `locations.jsonl`
List of all locations, `location_stats.location` references `country` or
`country-region`.

    {"country": "",   "region": "",   "country_name": "(unknown)",            "region_name": ""}
    {"country": "AE", "region": "",   "country_name": "United Arab Emirates", "region_name": ""}
    {"country": "DE", "region": "BW", "country_name": "Germany",              "region_name": "Baden-Württemberg"}


#### `paths.jsonl`
List of all paths. Events are also a "path" with `"event": true` (the `event`
field is omitted entirely for paths). Paths always start with a `/`; events
never start with a `/`.

This is referenced in all the statistics as `path_id`.

    {"id": 211,                    "path": "/",            "title": "Home page"}
    {"id": 215,                    "path": "/api",         "title": "API Documentation"}
    {"id": 1442298, "event": true, "path": "click-banana", "title": "Yellow curvy fruit"}

#### `refs.jsonl`
List of all referrers, referenced in `hit_stats` as `ref_id`. The `ref_scheme`
indicates the type of reference:
<ul>
<li><code>h</code>: HTTP (i.e. an URL).</li>
<li><code>g</code>: Generated; e.g. <code>mail.google.com</code> and several other email apps end up as <code>Email</code>.</li>
<li><code>c</code>: Campaign; text string from a campaign parameter.</li>
<li><code>o</code>: Other (e.g. Android apps).</li>
</ul>


    {"id": 1,        "ref": "",                       "ref_scheme": "o"}
    {"id": 69853,    "ref": "Google",                 "ref_scheme": "g"}
    {"id": 2176502,  "ref": "example.com/page.html",  "ref_scheme": "h"}

#### `browsers.jsonl`
A list of all browsers, as the browser name and version (may be blank). The
version is de-facto always a number (int or float), but it's not recommended to
rely on this.

    {"id": 1,   "name": "Chrome",   "version": "46"}
    {"id": 29,  "name": "",         "version": ""}
    {"id": 37,  "name": "Firefox",  "version": "83"}

#### `systems.jsonl`
A list of all operating systems, as the system name and version (may be blank).

    {"id": 2,   "name": "Android",  "version": "7.1"}
    {"id": 3,   "name": "Linux",    "version": "Ubuntu"}
    {"id": 18,  "name": "Windows",  "version": "10"}
    {"id": 39,  "name": "",         "version": ""}

### Statistics

#### `hit_stats.jsonl`
The main overview of hit counts, stored for every hour, path, and referral.

The `hour` is as RFC3339 and always in UTC; the minute and second is always 0.

You typically want to ignore the `ref_id` value for the main overview. For
example:

    select path_id, hour, sum(count) from hit_stats
    group by hour, path_id
    order by hour

<!-- -->

    {"hour": "2026-03-05T19:00:00Z",  "path_id": 211,       "ref_id": 1,         "count": 4}
    {"hour": "2026-03-07T15:00:00Z",  "path_id": 5990349,   "ref_id": 30563812,  "count": 1}
    {"hour": "2026-03-07T15:00:00Z",  "path_id": 82905422,  "ref_id": 30563812,  "count": 1}
    {"hour": "2026-03-07T15:00:00Z",  "path_id": 5990349,   "ref_id": 30563813,  "count": 3}
    {"hour": "2026-03-07T15:00:00Z",  "path_id": 82905422,  "ref_id": 30563814,  "count": 2}

#### `browser_stats.jsonl`
A list of all browser statistics for every day and path.

    {"day": "2019-08-15",  "path_id": 211,  "browser_id": 155,  "count": 1}
    {"day": "2019-08-15",  "path_id": 223,  "browser_id": 92,   "count": 6}
    {"day": "2019-08-15",  "path_id": 223,  "browser_id": 85,   "count": 1}

#### `system_stats.jsonl`
A list of all operating system statistics for every day and path.

    {"day": "2019-08-15",  "path_id": 211,  "system_id": 9,   "count": 4}
    {"day": "2019-08-15",  "path_id": 223,  "system_id": 52,  "count": 1}
    {"day": "2019-08-15",  "path_id": 211,  "system_id": 48,  "count": 2}

#### `location_stats.jsonl`
A list of all location statistics for every day and path. The `location`
references `location_stats.location` or
`location_stats.location-location_stats.location`.

    {"day": "2026-03-05",  "path_id": 211,  "location": "IN",     "count": 1}
    {"day": "2026-03-05",  "path_id": 211,  "location": "RO-B",   "count": 1}
    {"day": "2026-03-05",  "path_id": 211,  "location": "IT-82",  "count": 1}
    {"day": "2026-03-07",  "path_id": 350,  "location": "",       "count": 4}

#### `language_stats.jsonl`
A list of all operating system statistics for every day and path.

    {"day": "2026-03-05",  "path_id": 211,  "language": "eng", "count": 1}
    {"day": "2026-03-05",  "path_id": 211,  "language": "spa", "count": 2}
    {"day": "2026-03-05",  "path_id": 211,  "language": "",    "count": 2}

#### `size_stats.jsonl`
A list of all screen width statistics for every day and path.

    {"day": "2026-03-05",  "path_id": 211,      "width": 1280,  "count": 9}
    {"day": "2026-03-07",  "path_id": 5990349,  "width": 1920,  "count": 4}
    {"day": "2026-03-05",  "path_id": 5990350,  "width": 0,     "count": 1}

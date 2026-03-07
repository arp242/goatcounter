The JSON export is a ZIP file containing several JSON documents.

**Export from date**:
Statistics are stored with a resolution of an hour or date.

This means that if you create an export at 7:30 on March 10, and then another
one at 7:30 on March 20 with the start date set to March 10, you will have some
duplicate statistics for March 10.


Included files
--------------
.json files are complete JSON documents, with an object or array as the top
level, and may span multiple lines.

.jsonl files are in the "JSON lines" format. That is, every line contains a JSON object:

    {"key": "value"}
    {"key": "value"}
    {"key": "value"}

#### info.json
JSON object with information about the export:

    {
        "export_version":      1.0,
        "goatcounter_version": "2.7",
        "created_for":         "stats.arp242.net",
        "created_by":          "User 1 <martin@arp242.net>",
        "created_at":          "2026-03-07T16:49:27Z"
    }


#### languages.jsonl

    {"iso_639_3": "",    "name": "(unknown)"}
    {"iso_639_3": "aaa", "name": "Ghotuo"}
    {"iso_639_3": "aao", "name": "Algerian Saharan Arabic"}

#### locations.jsonl

    {"country": "",   "region": "",   "country_name": "(unknown)",            "region_name": ""}
    {"country": "AE", "region": "",   "country_name": "United Arab Emirates", "region_name": ""}
    {"country": "DE", "region": "BW", "country_name": "Germany",              "region_name": "Baden-Württemberg"}


#### paths.jsonl

    {"id": 211,     "event: false, "path": "/",            "title": "Home page"}
    {"id": 215,     "event: false, "path": "/api",         "title": "API Documentation"}
    {"id": 1442298, "event": true, "path": "click-banana", "title": "Yellow curvy fruit"}

#### refs.jsonl

    {"id": 1,        "ref": "",                       "ref_scheme": "o"}
    {"id": 69853,    "ref": "Google",                 "ref_scheme": "g"}
    {"id": 2176502,  "ref": "example.com/page.html",  "ref_scheme": "h"}

#### browsers.jsonl

    {"id": 1,   "name": "Chrome",   "version": "46"}
    {"id": 29,  "name": "",         "version": ""}
    {"id": 37,  "name": "Firefox",  "version": "83"}

#### systems.jsonl

    {"id": 2,   "name": "Android",  "version": "7.1"}
    {"id": 3,   "name": "Linux",    "version": "Ubuntu"}
    {"id": 18,  "name": "Windows",  "version": "10"}
    {"id": 39,  "name": "",         "version": ""}


#### browser_stats.jsonl

    {"day": "2019-08-15",  "path_id": 211,  "browser_id": 155,  "count": 1}
    {"day": "2019-08-15",  "path_id": 223,  "browser_id": 92,   "count": 6}
    {"day": "2019-08-15",  "path_id": 223,  "browser_id": 85,   "count": 1}

#### system_stats.jsonl

    {"day": "2019-08-15",  "path_id": 211,  "system_id": 9,   "count": 4}
    {"day": "2019-08-15",  "path_id": 223,  "system_id": 52,  "count": 1}
    {"day": "2019-08-15",  "path_id": 211,  "system_id": 48,  "count": 2}


#### location_stats.jsonl

    day
    path_id
    location
    count

#### language_stats.jsonl

    {"day": "2026-03-05",  "path_id": 211,      "location": "IN",     "count": 1}
    {"day": "2026-03-05",  "path_id": 211,      "location": "RO-B",   "count": 1}
    {"day": "2026-03-05",  "path_id": 211,      "location": "IT-82",  "count": 1}
    {"day": "2026-03-07",  "path_id": 5990349,  "location": "",       "count": 4}

#### size_stats.jsonl

    {"day": "2026-03-05",  "path_id": 211,      "width": 1280,  "count": 9}
    {"day": "2026-03-07",  "path_id": 5990349,  "width": 1920,  "count": 4}
    {"day": "2026-03-05",  "path_id": 5990350,  "width": 0,     "count": 1}

#### campaign_stats.jsonl

    day
    path_id
    campaign_id
    ref
    count

#### hit_stats.jsonl

    {"hour": "2026-03-05T19:00:00Z",  "path_id": 211,       "ref_id": 1,         "count": 4}
    {"hour": "2026-03-07T15:00:00Z",  "path_id": 5990349,   "ref_id": 30563812,  "count": 1}
    {"hour": "2026-03-07T15:00:00Z",  "path_id": 82905422,  "ref_id": 30563812,  "count": 1}
    {"hour": "2026-03-07T15:00:00Z",  "path_id": 5990349,   "ref_id": 30563813,  "count": 3}
    {"hour": "2026-03-07T15:00:00Z",  "path_id": 82905422,  "ref_id": 30563814,  "count": 2}

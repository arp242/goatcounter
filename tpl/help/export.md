The GoatCounter export is a CSV export of all pageviews of a site.

There is no "standard" CSV; the export is created with the [`encoding/csv`][csv]
package. Some notes:

- The first line is the header.
- Blank lines are allowed; lines may end with `\n` or `\r\n`.
- Optionally quote fields with `"`: `hello,"world"`
- Escape a `"` inside a quoted field by doubling it up: `hello,"""world"""`
- Newlines inside quoted fields are allowed.

[csv]: https://pkg.go.dev/encoding/csv#pkg-overview

CSV format
----------

The first line is a header with the field names. The fields, in order, are:

<table>
<tr><th>2,Path</th><td>Path name (e.g. <code>/a.html</code>).
    This also doubles as the event name. This header is prefixed
    with the version export format (see versioning below).</td></tr>
<tr><th>Title</th><td>Page title that was sent.</td></tr>
<tr><th>Event</th><td>If this is an event; <code>true</code> or <code>false</code>.</td></tr>
<tr><th>User-Agent</th><td>Always blank since the User-Agent is no longer stored.</td></tr>
<tr><th>Browser</th><td>Browser name and version.</td></tr>
<tr><th>System</th><td>System name and version.</td></tr>
<tr><th>Session</th><td>The session ID, to track unique visitors.</td>
<tr><th>Bot</th><td>If this is a bot request; <code>0</code> if it's
    not, or one of the
    <a href="https://pkg.go.dev/zgo.at/isbot?tab=doc#pkg-constants">isbot</a>
    constants if it is.</td></tr>
<tr><th>Referrer</th><td>Referrer data.</td></tr>
<tr><th>Referrer scheme</th><td>
        <code>h</code> – HTTP; an URL;<br>
        <code>g</code> – Generated; e.g. all the various Hacker News interfaces don't
        add a link to the specific story, so are just recorded as “Hacker News”;<br>
        <code>c</code> – Campaign; text string from a campaign parameter;<br>
        <code>o</code> – Other (e.g. Android apps).
    </td></tr>
<tr><th>Screen size</th><td>Screen size as <code>x,y,scaling</code>.</td></tr>
<tr><th>Location</th><td>ISO 3166-2 country code (either "US" or "US-TX")</td></tr>
<tr><th>FirstVisit</th><td>First visit in this session?</td>
<tr><th>Date</th><td>Creation date as RFC 3339/ISO 8601.</td></tr>
</table>

### Versioning
The format of the CSV file may change in the future; the version of the export
file is recorded at the start of the header as a number. It’s **strongly**
recommended to check this number if you're using a script to import/sync data
and error out if it changes. Any future incompatibilities will be documented
here.

<details>
<summary>Version 1 documentation</summary>

<p>The first line is a header with the field names. The fields, in order, are:</p>
<table>
<tr><th>1,Path</th><td>Path name (e.g. <code>/a.html</code>).
    This also doubles as the event name. This header is prefixed
    with the version export format (see versioning below).</td></tr>
<tr><th>Title</th><td>Page title that was sent.</td></tr>
<tr><th>Event</th><td>If this is an event; <code>true</code> or <code>false</code>.</td></tr>
<tr><th>Bot</th><td>If this is a bot request; <code>0</code> if it's
    not, or one of the
    <a href="https://pkg.go.dev/zgo.at/isbot?tab=doc#pkg-constants">isbot</a>
    constants if it is.</td></tr>
<tr><th>Session</th><td>The session ID, to track unique visitors.</td>
<tr><th>FirstVisit</th><td>First visit in this session?</td>
<tr><th>Referrer</th><td>Referrer data.</td></tr>
<tr><th>Referrer scheme</th><td>
        <code>h</code> – HTTP; an URL;<br>
        <code>g</code> – Generated; e.g. all the various Hacker News interfaces don't
        add a link to the specific story, so are just recorded as “Hacker News”;<br>
        <code>c</code> – Campaign; text string from a campaign parameter;<br>
        <code>o</code> – Other (e.g. Android apps).
    </td></tr>
<tr><th>Browser</th><td><code>User-Agent</code> header.</td></tr>
<tr><th>Screen size</th><td>Screen size as <code>x,y,scaling</code>.</td></tr>
<tr><th>Location</th><td>ISO 3166-1 country code.</td></tr>
<tr><th>Date</th><td>Creation date as RFC 3339/ISO 8601.</td></tr>
</table>
</details>


Importing in SQL
----------------

If you want to run analytics on this, you can use e.g. SQLite:

    sqlite> .import --csv gc_export.csv gc_export
    sqlite> select
       ...>   count(*) as count,
       ...>   substr(Location, 0, 3) as location
       ...> from gc_export
       ...> where location != ''
       ...> group by location
       ...> order by count desc
       ...> limit 20;
    ┌────────┬──────────┐
    │ count  │ location │
    ├────────┼──────────┤
    │ 113144 │ US       │
    │ 27092  │ DE       │
    │ 24131  │ GB       │
    │ 13269  │ CA       │
    │ 12977  │ FR       │
    │ 9785   │ NL       │
    │ 8150   │ IN       │
    │ 7487   │ AU       │
    │ 6864   │ PL       │
    │ 6760   │ SE       │
    └────────┴──────────┘

Or PostgreSQL:

    =# create table gc_export (
        "2Path"             varchar,
        "Title"             varchar,
        "Event"             varchar,
        "UserAgent"         varchar,
        "Browser"           varchar,
        "System"            varchar,
        "Session"           varchar,
        "Bot"               varchar,
        "Referrer"          varchar,
        "Referrer scheme"   varchar,
        "Screen size"       varchar,
        "Location"          varchar,
        "FirstVisit"        varchar,
        "Date"              varchar
    );

    =# \copy gc_export from 'gc_export.csv' with (format csv, header on);

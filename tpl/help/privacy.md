<style>
table      { width:auto; overflow-x:scroll;}
table.h td { border-left:1px solid #ddd; }
table + hr { margin-top:1rem; }
</style>

*8 June 2025* – Also see the [GDPR consent notices].

[GDPR consent notices]: /help/gdpr

What GoatCounter stores
-----------------------
GoatCounter stores information as separate "aggregate" tables per day or hour,
with no way to link data between them. It's probably easiest to explain by
example; for browsers it looks like:

| Path        | Day        | Browser     | Count |
| ----        | ---        | -------     | ----- |
| /page.html  | 2025-06-09 | Firefox 139 | 4     |
| /other.html | 2025-06-09 | Firefox 139 | 2     |
| /page.html  | 2025-06-09 | Chrome 106  | 1     |
| /page.html  | 2025-06-10 | Firefox 139 | 1     |
| /page.html  | 2025-06-10 | Chrome 106  | 3     |

We can see that on June 9th `/page.html` had 4 views with Firefox 139 and 1 with
Chrome 106, and that's all the information we have. Only the end result is
stored (browser name and version) and not the source where we retrieved it from
(`User-Agent` header).

And for example for screen widths:

| Path        | Day        | Width | Count |
| ----        | ---        | ----- | ----- |
| /page.html  | 2025-06-09 | 1920  | 1     |
| /other.html | 2025-06-09 | 400   | 2     |

There is no way to link this information with the browser data (or anything
else). We don't know if the person with the width of `1920` was using Firefox or
Chrome.

The following information is stored in this way:

- Browsers, based on headers sent by the browser (e.g. `Firefox 139`, `Chrome 102`)
- Systems, based on headers sent by the browser (e.g. `Windows 11`, `macOS 15.2`).
- Locations, based on IP address (e.g. `Indonesia`, `Canada`).
- Languages (e.g. `English`, `Klingon`).
- Screen widths (e.g. `768`, `1920`).

Sites may disable collecting any of this data; for example if you don't want to
collect screen sizes then you can disable it, and no screen sizes will be
stored.

---

For the pageviews and referrers data is stored per hour:

| Path        | Hour                | Count |
| ----        | ----                | ----- |
| /page.html  | 2025-06-09 10:00:00 | 4     |
| /other.html | 2025-06-09 10:00:00 | 2     |
| /page.html  | 2025-06-09 11:00:00 | 1     |

| Path        | Referrer                     | Hour                | Count |
| ----        | --------                     | ----                | ----- |
| /page.html  | http://example.com/page.html | 2025-06-09 10:00:00 | 1     |
| /other.html | custom-referrer              | 2025-06-09 10:00:00 | 2     |

Other than that, it's identical to the data stored per day.

---

GoatCounter includes an option to collect individual pageviews (disabled by
default). If this is enabled it will also record a row for every pageview (in
addition to the above):

<table class="h">
<tr><th>Time</th>     <td>2025-06-09 19:05:05</td>  <td>2025-06-09 19:07:59</td>    </tr>
<tr><th>Path</th>     <td>/page.html</td>           <td>/other.html</td>            </tr>
<tr><th>Referrer</th> <td>https://example.com</td>  <td>-</td>                      </tr>
<tr><th>Session</th>  <td>aed10f81</td>             <td>aed10f81</td>               </tr>
<tr><th>Entry</th>    <td>yes</td>                  <td>no</td>                     </tr>
<tr><th>Bot</th>      <td>no</td>                   <td>no</td>                     </tr>
<tr><th>Browser</th>  <td>Firefox 139</td>          <td>Firefox 139</td>            </tr>
<tr><th>System</th>   <td>Windows 11</td>           <td>Windows 11</td>             </tr>
<tr><th>Width</th>    <td>1920</td>                 <td>1920</td>                   </tr>
<tr><th>Location</th> <td>Ireland</td>              <td>Ireland</td>                </tr>
<tr><th>Language</th> <td>eng</td>                  <td>eng</td>                    </tr>
</table>

There is not a lot of extra information here as such, but it does give a
slightly more detailed view than just the aggregate statistics.

To keep track of repeated visits, GoatCounter stores the site name + IP +
`User-Agent` in memory for up to 8 hours. This is not stored in the database,
instead it stores a random generated string. The mapping of `site + IP +
User-Agent` to random ID is only stored in memory (for up to eight hours). This
random string is only stored in the database if collection of individual
pageviews is enable. Otherwise it doesn't store anything (it simply won't count
if we know about the session). See [Sessions and visitors] for a slightly more
detailed overview.

In short, GoatCounter *doesn't* store IP addresses, the full User-Agent header,
or any tracker ID. It also doesn't store any information in the browser with
cookies, localStorage, cache, or any other method.

[Sessions and visitors]: /help/sessions

Sharing with third parties
--------------------------
No information is shared with third parties.

Using the GoatCounter.com service
---------------------------------
An email address is required to use the GoatCounter.com service. GoatCounter
also use cookies to:

- remember that you’re logged in to your account between visits;
- store short-lived informational messages (“flash messages”), for example to
  inform that an operation was completed or that there was an error.

Data is stored on servers at Hetzner Online GmbH in Finland and Germany.
GoatCounter.com is operated by Martin Tournoij, located in Ireland.

You can remove all data from the site settings (`Settings → Delete Account`),
which will permanently delete all data. Some data may be retained in backups for
up to 30 days.

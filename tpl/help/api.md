The GoatCounter API can be used to manage sites, users, export data, or build
your own dashboard.

The API is currently unversioned and prefixed with `/api/v0`; breaking changes
will be avoided and are not expected but *may* occur. I'll be sure to send ample
notification of this to everyone who has generated an API key.


Authentication
--------------
To use the API create a key in your account (`[Username in top menu] → API`); send the API key in
the `Authorization` header as `Authorization: Bearer [token]`. You will need to
use `Content-Type: application/json`; all requests return JSON unless noted
otherwise.

Example:

    curl https://example.goatcounter.com/api/v0/me \
        -H 'Content-Type: application/json' \
        -H 'Authorization: Bearer 2q2snk7clgqs63tr4xc5bwseajlw88qzilr8fq157jz3qxwwmz5'

Replace the key and URL with your actual values.

HTTP Basic auth is also supported; this is mostly useful for testing things from
the browser. Leave the username empty and use the API key as the password.

Endpoints return `401 Unauthorized` if the API key is missing or incorrect, and
`403 Forbidden` if it doesn't have the needed permissions.

Rate limit
----------
The rate limit is 4 requests per second; the current values are reported in the
response headers:

    X-Rate-Limit-Limit        Number of requests the rate limit kicks in; this is always the same.
    X-Rate-Limit-Remaining    Requests remaining this period.
    X-Rate-Limit-Reset        Seconds until the rate limits resets.


Errors
------
Errors are reported in either an `error` or `errors` field; the `error` field
always contains a string; for example:

    {
        "error": "oh noes!"
    }

The `errors` field contains an object with a list:

    {
        "errors": {
            "key":     ["error1", "error2"],
            "another": ["oh noes!"]
        }
    }

A status code in the `2xx` range will never contain errors, a status code in the
`4xx` or `5xx` range will always have either `error` or `errors`, but never
both. There may also be additional data in other fields on errors.

API reference
-------------
API reference docs are available at:

- [/api.json]({{.Base}}/api.json) – OpenAPI 2.0 JSON file.
- Online viewer: [RapiDoc][1], [SwaggerHub][2] <!-- too broken for now  [simple HTML][3] -->

[1]: /api2.html
[2]: https://app.swaggerhub.com/apis-docs/Carpetsmoker/GoatCounter/0.1
[3]: /api.html

Quick overview
--------------
A quick overview of all the endpoints:

|                                      |                                        |
| ----                                 | -----                                  |
| `POST  /api/v0/count`                | Count pageviews                        |
| **Exports**                          |                                        |
| `POST  /api/v0/export`               | Create a new CSV export                |
| `GET   /api/v0/export/{id}`          | Get information about a CSV export     |
| `GET   /api/v0/export/{id}/download` | Download CSV export                    |
| **Statistics**                       |                                        |
| `GET   /api/v0/stats/total`          | List total pageview counts             |
| `GET   /api/v0/stats/hits`           | Get pageview and visitor statistics    |
| `GET   /api/v0/stats/hits/{path_id}` | Get referral stats for a path          |
| `GET   /api/v0/stats/{page}`         | Get stats for browser, system, etc.    |
| `GET   /api/v0/stats/{page}/{id}`    | Detailed stats (e.g. browser version)  |
| **Sites**                            |                                        |
| `GET   /api/v0/sites`                | List sites                             |
| `PUT   /api/v0/sites`                | Create a new site                      |
| `GET   /api/v0/sites/{id}`           | Detailed information about a site      |
| `POST  /api/v0/sites/{id}`           | Update a site                          |
| `PATCH /api/v0/sites/{id}`           | Update a site                          |
| **Users**                            |                                        |
| `GET   /api/v0/me`                   | Get information about the current user |
| **Paths**                            |                                        |
| `GET   /api/v0/paths`                | Get an overview of all paths           |

<style>table code { white-space: pre-wrap; background-color: inherit; }</style>

Examples
--------
A few shell script examples for common use cases; all of these require [curl],
and some require [jq].

[curl]: https://curl.se/
[jq]: https://stedolan.github.io/jq/

### Sending pageviews from a backend
You can use `/api/v0/count` to send pageviews from your backend. This is the
same as `/count` that the JavaScript integration uses but has higher
rate-limits, allows setting some additional fields, and allows batching multiple
pageviews in one request.

A simple example might look like:

    {{template "sh_header" .}}

    curl -X POST  "$api/count" \
        --data '{"no_sessions": true, "hits": [{"path": "/one"}, {"path": "/two"}]}'

### Exporting to CSV
Example to export via the API:

    {{template "sh_header" .}}

    # Start a new export, get ID from response object.
    id=$(curl -X POST "$api/export" | jq .id)

    # The export is started in the background, so we'll need to wait until it's finished.
    while :; do
        sleep 1

        finished=$(curl "$api/export/$id" | jq .finished_at)
        if [ "$finished" != "null" ]; then
            # Download the export.
            curl "$api/export/$id/download" | gzip -d

            break
        fi
    done

The above does no error checking for brevity: errors are reported in the `error`
or `errors` field as described in the earlier section.

The export object contains a `last_hit_id` parameter, which can be used as a
pagination cursor to only download hits after this export. This is useful to
sync your local database regularly:

    # Get cursor
    start=$(curl "$api/export/$id" | jq .last_hit_id)

    # Start new export starting from the cursor.
    id=$(curl -X POST "$api/export" --data "{\"start_from_hit_id\":$start}" | jq .id)

### Loading statistics
With the `/api/v0/stats/*` endpoint you get retrieve the dashboard statistics.

An example is available as the [`goatcounter dashboard`][dashboard] command, as
a (POSIX) shell script would probably be too convoluted to be useful.

In the example it gets all the data in serial for simplicity, but in more
serious applications you probably want to get a few of them in parallel; this is
also what the default GoatCounter dashboard does.

[dashboard]: https://github.com/arp242/goatcounter/blob/master/cmd/goatcounter/dashboard.go

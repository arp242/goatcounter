{{template "%%top.gohtml" .}}
{{define "sh_header"}}#!/bin/sh<br>
token=\[your api token]
api=https://\[my code].goatcounter.com/api/v0
curl() {
    \command curl \
        -H 'Content-Type: application/json' \
        -H "Authorization: Bearer $token" \
        "$@"
}{{end}}

GoatCounter API
===============
GoatCounter has a rudimentary API; this is far from feature-complete, but solves
some common use cases.

The API is currently unversioned and prefixed with `/api/v0`; breaking changes
will be avoided and are not expected but *may* occur. I'll be sure to send ample
notification of this to everyone who has generated an API key.


Authentication
--------------
To use the API create a key in your account (`Settings → Password, MFA, API`);
send the API key in the `Authorization` header as `Authorization: bearer
[token]`.

You will need to use `Content-Type: application/json`; all requests return JSON
unless noted otherwise.

Example:

    curl -X POST https://example.goatcounter.com/api/v0/export \
        -H 'Content-Type: application/json' \
        -H 'Authorization: Bearer 2q2snk7clgqs63tr4xc5bwseajlw88qzilr8fq157jz3qxwwmz5'

Replace the key and URL with your actual values.


Rate limit
----------
The rate limit is 60 requests per 120 seconds. The current rate limits are
indicated in the `X-Rate-Limit-Limit`, `X-Rate-Limit-Remaining`, and
`X-Rate-Limit-Reset` headers.


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

- [/api.json](/api.json) – OpenAPI 2.0 JSON file.
- [/api.html](/api.html) – Basic HTML (still kinda ugly 😅).
- [SwaggerHub](https://app.swaggerhub.com/apis-docs/Carpetsmoker/GoatCounter/0.1)

The files are also available in the `docs` directory of the source repository.


Examples
--------

### Backend integration
You can use `/api/v0/count` to send requests from your backend. This is the same
as `/count` but has higher rate-limits, allows setting some additional fields,
and allows batching multiple pageviews in one request.

Detail are available in the [API reference](/api.html#count), a simple example
might look like:

    {{template "sh_header" .}}

    curl -X POST  "$api/count" \
        --data '{"no_sessions": true, "hits": [{"path": "/one"}, {"path": "/two"}]}'

### Export

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

The above doesn't does no error checking for brevity: errors are reported in the
`error` or `errors` field as described above.

The export object contains a `last_hit_id` parameter, which can be used as a
pagination cursor to only download hits after this export. This is useful to
sync your local database every hour or so:

    # Get cursor
    start=$(curl "$api/export/$id" | jq .last_hit_id)

    # Start new export starting from the cursor.
    id=$(curl -X POST "$api/export" --data "{\"start_from_hit_id\":$start}" | jq .id)

{{template "%%bottom.gohtml" .}}

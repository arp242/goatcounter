{{template "%%top.gohtml" .}}

GoatCounter API
===============
GoatCounter has a rudimentary API; this is far from feature-complete, but solves
some common use cases.

The API is currently unversioned and prefixed with `/api/v0`; while breaking
changes will be avoided and are not expected, they *may* occur. I'll be sure to
send ample notification of this to everyone who has generated an API key.

Authentication
--------------
To use the API create a key in your account (`Settings → Password, MFA, API`);
send the API key in the `Authorization` header as `Authorization: bearer
[token]`.

You will need to use `Content-Type: application/json`; all requests return JSON
unless noted otherwise.

Example:

    curl -X POST \
        -H 'Content-Type: application/json' \
        -H 'Authorization: Bearer 2q2snk7clgqs63tr4xc5bwseajlw88qzilr8fq157jz3qxwwmz5' \
        https://example.goatcounter.com/api/v0/export

Rate limit
----------
The rate limit is 60 requests per 120 seconds. The current rate limits are
indicated in the `X-Rate-Limit-Limit`, `X-Rate-Limit-Remaining`, and
`X-Rate-Limit-Reset` headers.

API reference
-------------
API reference docs are available at:

- [/api.json](/api.json) – OpenAPI 2.0 JSON file.
- [/api.html](/api.html) – Basic HTML.
- [SwaggerHub](https://app.swaggerhub.com/apis-docs/Carpetsmoker/GoatCounter/0.1)

The files are also available in the `docs` directory of the source repository.

Examples
--------

### Export

Example to export via the API:

    token=[your api token]
    api=http://[my code].goatcounter.com/api/v0
    curl() {
        \command curl --silent \
            -H 'Content-Type: application/json' \
            -H "Authorization: Bearer $token" \
            $@
    }

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
`error` field as a string, or in the `errors` field as `{"name": ["err1",
"err2", "name2": [..]}`.

The export object contains a `last_hit_id` parameter, which can be used as a
pagination cursor to only download hits after this export. This is useful to
sync your local database every hour or so:

    # Get cursor
    start=$(curl "$api/export/$id" | jq .last_hit_id)

    # Start new export starting from the cursor.
    id=$(curl -X POST --data "{\"start_from_hit_id\":$start}" "$api/export" | jq .id)

{{template "%%bottom.gohtml" .}}

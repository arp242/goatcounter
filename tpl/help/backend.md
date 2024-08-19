You can use the `/api/v0/count` API endpoint to send pageviews from essentially
anywhere, such as your app's middleware.

A simple example from `curl`:

    token=[your api token]
    api={{.SiteURL}}/api/v0

    curl -X POST "$api/count" \
        -H 'Content-Type: application/json' \
        -H "Authorization: Bearer $token" \
        --data '{"no_sessions": true, "hits": [{"path": "/one"}, {"path": "/two"}]}'

The [API documentation]({{.Base}}/api) contains detailed information and more examples.

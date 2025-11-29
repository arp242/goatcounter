GoatCounter *does*, optionally, store the domain a pageview belongs to;
If the log importer script is run with `$host` in the log format, or uses a
format that includes that, such as `combined-vhost`, `common-vhost`, `bunny`.

Currently, javascript api `count.js`, does not support doing this.

To report a host via the raw API include the `host` field within the `hits` list, such as:

```
curl -X POST "$api/count" \
    -H 'Content-Type: application/json' \
    -H "Authorization: Bearer $token" \
    --data '{"no_sessions": true, "hits": [{"host": "example.org", "path": "/one"}]}'
```

Software Bill of Materials
==========================

Maintained here as part of the [NGI0 PET Fund](https://nlnet.nl/PET/)
application.

Backend
-------

Direct dependencies:

| Name                                 | License      | Why?                                                  |
| ----                                 | -------      | -----                                                 |
| code.soquee.net/otp                  | BSD-2-Clause | Generate tokens for MFA                               |
| github.com/bmatcuk/doublestar/v3     | MIT          | -exclude 'glob:..' flag in `goatcounter import`.      |
| github.com/boombuler/barcode         | MIT          | Generating the QR code for MFA                        |
| github.com/go-chi/chi                | MIT          | HTTP routing                                          |
| github.com/google/uuid               | BSD-3-Clause | Generate UUIDs for sessions.                          |
| github.com/jinzhu/now                | MIT          | Make some date calculations easier.                   |
| github.com/lib/pq                    | MIT          | PostgreSQL database support                           |
| github.com/mattn/go-sqlite3          | MIT          | SQLite database support                               |
| github.com/monoculum/formam          | Apache-2.0   | Decode HTTP forms to Go structs.                      |
| github.com/oschwald/geoip2-golang    | ISC          | Get location from IP address.                         |
| github.com/oschwald/maxminddb-golang | ISC          | Get Location from IP address.                         |
| github.com/russross/blackfriday      | BSD-2-Clause | Some pages are in Markdown                            |
| github.com/teamwork/reload           | MIT          | Automatically reload                                  |
| golang.org/x/crypto                  | BSD-3-Clause | Hash passwords, create TLS certs for ACME.            |
| golang.org/x/image                   | BSD-3-Clause | Create PNG version of the "visitor counter".          |
| golang.org/x/net                     | BSD-3-Clause | Generate and validate TOTP tokens.                    |
| golang.org/x/sync                    | BSD-3-Clause | x/sync/singleflight                                   |
| zgo.at/blackmail                     | MIT          | Send emails.                                          |
| zgo.at/errors                        | MIT          | More convenient errors.                               |
| zgo.at/follow                        | MIT          | "goatcounter import -follow"                          |
| zgo.at/gadget                        | MIT          | Get browser and system name and name from User-Agent. |
| zgo.at/guru                          | MIT          | Errors with a status code.                            |
| zgo.at/isbot                         | MIT          | Detect bots.                                          |
| zgo.at/json                          | MIT          | encoding/json with ,readonly tag                      |
| zgo.at/tz                            | MIT          | Present timezones in dropdown nicely.                 |
| zgo.at/zcache                        | MIT          | In-memory caching.                                    |
| zgo.at/zdb                           | MIT          | Database access layer                                 |
| zgo.at/zhttp                         | MIT          | HTTP tools                                            |
| zgo.at/zli                           | MIT          | CLI conveniences                                      |
| zgo.at/zlog                          | MIT          | Logging library.                                      |
| zgo.at/zstd                          | MIT          | Extensions to stdlib.                                 |
| zgo.at/zstripe                       | MIT          | Stripe integration                                    |
| zgo.at/zvalidate                     | MIT          | Validate values                                       |

Testing dependencies:

| github.com/PuerkitoBio/goquery | BSD 3-Clause | Used in tests to check the HTML |
| golang.org/x/tools             | BSD-3-Clause | Linting tools. |
| honnef.co/go/tools             | MIT          | Linting tools. |

Indirect transient dependencies; not these may not actually be used/compiled in:

| Name                            | License      |
| ----                            | -------      |
| github.com/BurntSushi/toml      | MIT          |
| github.com/andybalholm/cascadia | BSD-2-Clause |
| github.com/davecgh/go-spew      | ISC          |
| github.com/fsnotify/fsnotify    | BSD-3-Clause |
| github.com/go-sql-driver/mysql  | MPL-2.0      |
| github.com/kisielk/gotool       | MIT          |
| github.com/pmezard/go-difflib   | BSD-3-Clause |
| github.com/stretchr/objx        | MIT          |
| github.com/stretchr/testify     | MIT          |
| github.com/yuin/goldmark        | MIT          |
| golang.org/x/mod                | BSD-3-Clause |
| golang.org/x/sys                | BSD-3-Clause |
| golang.org/x/term               | BSD-3-Clause |
| golang.org/x/text               | BSD-3-Clause |
| golang.org/x/xerrors            | BSD-3-Clause |
| gopkg.in/check.v1               | BSD-2-Clause |
| gopkg.in/yaml.v3                | Apache-2.0   |

Frontend:

| Name    | License | Why?                                                                                                                           |
| ----    | ------- | ----                                                                                                                           |
| jQuery  | MIT     | It's just easier than the DOM.                                                                                                 |
| Pikaday | MIT     | Date picker; native browser date pickers don't actually work all that well (even in 2020), and this provides a much better UX. |
| Dragula | MIT     | Drag & drop; native browser functionality isn't broadly supported.                                                             |

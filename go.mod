module zgo.at/goatcounter

go 1.13

// "Fork" of go-sqlite3 which removes the sqlite_json build constraint, so it
// compiles with JSON support without having to specify a build tag, which is
// inconvenient, easily forgotten, and causes runtime errors.
replace github.com/mattn/go-sqlite3 => github.com/zgoat/go-sqlite3 v1.14.6-json

// https://github.com/oschwald/maxminddb-golang/pull/75
replace github.com/oschwald/maxminddb-golang => github.com/zgoat/maxminddb-golang v1.8.1-0.20201227124339-dc03187a9664

// https://github.com/oschwald/geoip2-golang/pull/68
replace github.com/oschwald/geoip2-golang => github.com/zgoat/geoip2-golang v1.4.1-0.20201227124715-9eb17ed0da06

// https://github.com/jmoiron/sqlx/pull/680
replace github.com/jmoiron/sqlx => github.com/zgoat/sqlx v1.2.1-0.20201228123424-c5cc0d957b92

require (
	code.soquee.net/otp v0.0.1
	github.com/PuerkitoBio/goquery v1.6.0
	github.com/boombuler/barcode v1.0.0
	github.com/go-chi/chi v1.5.1
	github.com/google/uuid v1.1.3
	github.com/jinzhu/now v1.1.1
	github.com/jmoiron/sqlx v1.2.1-0.20201120164427-00c6e74d816a
	github.com/lib/pq v1.9.0
	github.com/mattn/go-sqlite3 v1.14.6
	// https://github.com/monoculum/formam/pull/38
	github.com/monoculum/formam v0.0.0-20201224092534-2a1a2c48fe6d
	github.com/oschwald/geoip2-golang v1.4.0
	github.com/oschwald/maxminddb-golang v1.8.0
	github.com/teamwork/reload v1.3.2
	golang.org/x/crypto v0.0.0-20201208171446-5f87f3452ae9
	golang.org/x/image v0.0.0-20201208152932-35266b937fa6
	golang.org/x/net v0.0.0-20201209123823-ac852fbbde11
	golang.org/x/sync v0.0.0-20201207232520-09787c993a3a
	golang.org/x/term v0.0.0-20201210144234-2321bbc49cbf
	golang.org/x/tools v0.0.0-20201211185031-d93e913c1a58
	honnef.co/go/tools v0.1.0
	zgo.at/blackmail v0.0.0-20200703094839-f1e44ef1dbb8
	zgo.at/errors v1.0.0
	zgo.at/gadget v0.0.0-20201217063255-80176bd17067
	zgo.at/guru v1.1.0
	zgo.at/isbot v0.0.0-20201217063241-a1aab44f6889
	zgo.at/json v0.0.0-20200627042140-d5025253667f
	zgo.at/tz v0.0.0-20201224084217-b40a2f90fff3
	zgo.at/zcache v1.0.1-0.20201224082040-4b746633475e
	zgo.at/zdb v0.0.0-20201230221703-1c334f9465a5
	zgo.at/zhttp v0.0.0-20201222222554-9c9e1d2d6f2c
	zgo.at/zli v0.0.0-20210102181013-33768b083e81
	zgo.at/zlog v0.0.0-20201213081304-1dc74ce06e5f
	zgo.at/zpack v1.0.2-0.20201215095635-1a4d171dcd00
	zgo.at/zstd v0.0.0-20210102201316-f587f1da7d03
	zgo.at/zstripe v1.0.0
	zgo.at/zvalidate v0.0.0-20201227171559-09b756b3b132
)

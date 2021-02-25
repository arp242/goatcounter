module zgo.at/goatcounter

go 1.16

// "Fork" of go-sqlite3 which removes the sqlite_json build constraint, so it
// compiles with JSON support without having to specify a build tag, which is
// inconvenient, easily forgotten, and causes runtime errors.
replace github.com/mattn/go-sqlite3 => github.com/zgoat/go-sqlite3 v1.14.6-json

// https://github.com/oschwald/maxminddb-golang/pull/75
replace github.com/oschwald/maxminddb-golang => github.com/zgoat/maxminddb-golang v1.8.1-0.20201227124339-dc03187a9664

// https://github.com/oschwald/geoip2-golang/pull/68
replace github.com/oschwald/geoip2-golang => github.com/zgoat/geoip2-golang v1.4.1-0.20201227124715-9eb17ed0da06

require (
	code.soquee.net/otp v0.0.1
	github.com/PuerkitoBio/goquery v1.6.1
	github.com/boombuler/barcode v1.0.1
	github.com/go-chi/chi v1.5.3
	github.com/google/uuid v1.2.0
	github.com/jinzhu/now v1.1.1
	github.com/jmoiron/sqlx v1.3.1
	github.com/lib/pq v1.9.0
	github.com/mattn/go-sqlite3 v1.14.6
	github.com/monoculum/formam v0.0.0-20210131081218-41b48e2a724b
	github.com/oschwald/geoip2-golang v1.4.0
	github.com/oschwald/maxminddb-golang v1.8.0
	github.com/teamwork/reload v1.3.2
	golang.org/x/crypto v0.0.0-20210220033148-5ea612d1eb83
	golang.org/x/image v0.0.0-20210220032944-ac19c3e999fb
	golang.org/x/net v0.0.0-20210222171744-9060382bd457
	golang.org/x/sync v0.0.0-20210220032951-036812b2e83c
	golang.org/x/term v0.0.0-20210220032956-6a3ed077a48d
	golang.org/x/tools v0.1.0
	honnef.co/go/tools v0.1.2
	zgo.at/blackmail v0.0.0-20200703094839-f1e44ef1dbb8
	zgo.at/errors v1.0.0
	zgo.at/gadget v0.0.0-20201217063255-80176bd17067
	zgo.at/guru v1.1.0
	zgo.at/isbot v0.0.0-20201217063241-a1aab44f6889
	zgo.at/json v0.0.0-20200627042140-d5025253667f
	zgo.at/tz v0.0.0-20201224084217-b40a2f90fff3
	zgo.at/zcache v1.0.1-0.20201224082040-4b746633475e
	zgo.at/zdb v0.0.0-20210225035815-96c4e1992cc3
	zgo.at/zhttp v0.0.0-20201222222554-9c9e1d2d6f2c
	zgo.at/zli v0.0.0-20210102181013-33768b083e81
	zgo.at/zlog v0.0.0-20201213081304-1dc74ce06e5f
	zgo.at/zpack v1.0.2-0.20201215095635-1a4d171dcd00
	zgo.at/zstd v0.0.0-20210225035742-8cba6f19cb00
	zgo.at/zstripe v1.0.0
	zgo.at/zvalidate v0.0.0-20201227171559-09b756b3b132
)

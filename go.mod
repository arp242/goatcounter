module zgo.at/goatcounter

go 1.13

// "Fork" of go-sqlite3 which removes the sqlite_json build constraint, so it
// compiles with JSON support without having to specify a build tag, which is
// inconvenient, easily forgotten, and causes runtime errors.
replace github.com/mattn/go-sqlite3 => github.com/zgoat/go-sqlite3 v1.14.5-json

require (
	code.soquee.net/otp v0.0.1
	github.com/PuerkitoBio/goquery v1.6.0
	github.com/arp242/geoip2-golang v1.4.0
	github.com/boombuler/barcode v1.0.0
	github.com/go-chi/chi v1.5.1
	github.com/google/uuid v1.1.2
	github.com/jinzhu/now v1.1.1
	github.com/jmoiron/sqlx v1.2.0
	github.com/lib/pq v1.9.0
	github.com/mattn/go-sqlite3 v1.14.5
	github.com/monoculum/formam v0.0.0-20200923020755-6f187e4ffe27
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
	zgo.at/pg_info v0.0.0-20201217021255-048639cbc5d4
	zgo.at/tz v0.0.0-20201224084217-b40a2f90fff3
	zgo.at/zcache v1.0.1-0.20201224082040-4b746633475e
	zgo.at/zdb v0.0.0-20201219034513-6c6c7c592e1c
	zgo.at/zhttp v0.0.0-20201221031833-2d4ffd61ee72
	zgo.at/zli v0.0.0-20201210061107-1204fda2cf4b
	zgo.at/zlog v0.0.0-20201213081304-1dc74ce06e5f
	zgo.at/zpack v1.0.2-0.20201215095635-1a4d171dcd00
	zgo.at/zstd v0.0.0-20201219074540-1bdc62c5acc9
	zgo.at/zstripe v1.0.0
	zgo.at/zvalidate v0.0.0-20200611174908-64a702efab5a
)

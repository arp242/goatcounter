module zgo.at/goatcounter/v2

go 1.17

require (
	code.soquee.net/otp v0.0.1
	github.com/BurntSushi/toml v0.4.1
	github.com/PuerkitoBio/goquery v1.8.0
	github.com/bmatcuk/doublestar/v3 v3.0.0
	github.com/boombuler/barcode v1.0.1
	github.com/go-chi/chi/v5 v5.0.7
	github.com/google/uuid v1.3.0
	github.com/gorilla/websocket v1.4.2
	github.com/mattn/go-sqlite3 v1.14.9
	github.com/monoculum/formam v3.5.5+incompatible
	github.com/oschwald/geoip2-golang v1.4.0
	github.com/russross/blackfriday/v2 v2.1.0
	github.com/teamwork/reload v1.3.2
	golang.org/x/crypto v0.0.0-20211215153901-e495a2d5b3d3
	golang.org/x/image v0.0.0-20211028202545-6944b10bf410
	golang.org/x/net v0.0.0-20211216030914-fe4d6282115f
	golang.org/x/sync v0.0.0-20210220032951-036812b2e83c
	golang.org/x/text v0.3.7
	golang.org/x/tools v0.1.8
	honnef.co/go/tools v0.2.2
	zgo.at/blackmail v0.0.0-20211212060815-1f8e8a94692b
	zgo.at/errors v1.1.0
	zgo.at/follow v0.0.0-20211017230838-112008350298
	zgo.at/gadget v0.0.0-20211017230912-e9a0ecc62867
	zgo.at/guru v1.1.0
	zgo.at/isbot v0.0.0-20211017231009-742e7be1c6d8
	zgo.at/json v0.0.0-20211017213340-cc8bf51df08c
	zgo.at/tz v0.0.0-20211017223207-006eae99adf6
	zgo.at/z18n v0.0.0-20211201221236-c1ccdacc3808
	zgo.at/zcache v1.0.1-0.20210412145246-76039d792310
	zgo.at/zdb v0.0.0-20220113132609-caba6bb9c06e
	zgo.at/zhttp v0.0.0-20211213094732-dd554e63f604
	zgo.at/zli v0.0.0-20211215141047-76dae1509b03
	zgo.at/zlog v0.0.0-20211008102840-46c1167bf2a9
	zgo.at/zprof v0.0.0-20211217104121-c3c12596d8f0
	zgo.at/zstd v0.0.0-20211220090423-fa07dcab58f8
	zgo.at/zstripe v1.1.1-0.20210407063143-62ac9deebc08
	zgo.at/ztpl v0.0.0-20211128061406-6ff34b1256c4
	zgo.at/zvalidate v0.0.0-20211128195927-d13b18611e62
)

require (
	github.com/andybalholm/cascadia v1.3.1 // indirect
	github.com/fsnotify/fsnotify v1.4.9 // indirect
	github.com/go-sql-driver/mysql v1.6.0 // indirect
	github.com/lib/pq v1.10.4 // indirect
	github.com/oschwald/maxminddb-golang v1.8.0 // indirect
	golang.org/x/mod v0.5.1 // indirect
	golang.org/x/sys v0.0.0-20211019181941-9d821ace8654 // indirect
	golang.org/x/term v0.0.0-20210317153231-de623e64d2a6 // indirect
	golang.org/x/xerrors v0.0.0-20200804184101-5ec99f83aff1 // indirect
)

// "Fork" of go-sqlite3 which removes the sqlite_json build constraint so it
// compiles with JSON support without having to specify a build tag.
replace github.com/mattn/go-sqlite3 => github.com/arp242/go-sqlite3 v1.13.1-0.20220110230139-61304ade21a9

// https://github.com/oschwald/maxminddb-golang/pull/75
replace github.com/oschwald/maxminddb-golang => github.com/arp242/maxminddb-golang v1.8.1-0.20201227124339-dc03187a9664

// https://github.com/oschwald/geoip2-golang/pull/68
replace github.com/oschwald/geoip2-golang => github.com/arp242/geoip2-golang v1.4.1-0.20201227124715-9eb17ed0da06

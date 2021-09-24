module zgo.at/goatcounter/v2

go 1.17

require (
	code.soquee.net/otp v0.0.1
	github.com/PuerkitoBio/goquery v1.6.1
	github.com/bmatcuk/doublestar/v3 v3.0.0
	github.com/boombuler/barcode v1.0.1
	github.com/go-chi/chi/v5 v5.0.4
	github.com/google/uuid v1.3.0
	github.com/jinzhu/now v1.1.2
	github.com/mattn/go-sqlite3 v1.14.6
	github.com/monoculum/formam v0.0.0-20210131081218-41b48e2a724b
	github.com/oschwald/geoip2-golang v1.4.0
	github.com/russross/blackfriday/v2 v2.1.0
	github.com/teamwork/reload v1.3.2
	golang.org/x/crypto v0.0.0-20210915214749-c084706c2272
	golang.org/x/image v0.0.0-20210628002857-a66eb6448b8d
	golang.org/x/net v0.0.0-20210917221730-978cfadd31cf
	golang.org/x/sync v0.0.0-20210220032951-036812b2e83c
	golang.org/x/text v0.3.7
	golang.org/x/tools v0.1.6
	honnef.co/go/tools v0.2.1
	zgo.at/blackmail v0.0.0-20210321151525-a218c2f584be
	zgo.at/errors v1.0.1-0.20210313142254-4e0fb19b1249
	zgo.at/follow v0.0.0-20201229040459-c683c36702b6
	zgo.at/gadget v0.0.0-20210225052028-befd29935cb7
	zgo.at/guru v1.1.0
	zgo.at/isbot v0.0.0-20210512054941-d1f89ea37986
	zgo.at/json v0.0.0-20200627042140-d5025253667f
	zgo.at/tz v0.0.0-20210320184244-8641ea282782
	zgo.at/z18n v0.0.0-20210628021318-311bb2053a95
	zgo.at/zcache v1.0.1-0.20210412145246-76039d792310
	zgo.at/zdb v0.0.0-20210512041154-a6be15a82747
	zgo.at/zhttp v0.0.0-20210521121346-91e65b54cd22
	zgo.at/zli v0.0.0-20210625065259-d03e49b7c9ea
	zgo.at/zlog v0.0.0-20210403053344-79deb263f0d9
	zgo.at/zprof v0.0.0-20210408083551-44ef6d69c2ec
	zgo.at/zstd v0.0.0-20210628014301-6fe5ffd0474c
	zgo.at/zstripe v1.1.1-0.20210407063143-62ac9deebc08
	zgo.at/ztpl v0.0.0-20210522104216-89fb2373a16b
	zgo.at/zvalidate v0.0.0-20210627041111-7942ce72d87f
)

require (
	github.com/BurntSushi/toml v0.3.2-0.20210624061728-01bfc69d1057 // indirect
	github.com/andybalholm/cascadia v1.1.0 // indirect
	github.com/fsnotify/fsnotify v1.4.9 // indirect
	github.com/go-sql-driver/mysql v1.5.0 // indirect
	github.com/jmoiron/sqlx v1.3.1 // indirect
	github.com/lib/pq v1.9.0 // indirect
	github.com/oschwald/maxminddb-golang v1.8.0 // indirect
	golang.org/x/mod v0.4.2 // indirect
	golang.org/x/sys v0.0.0-20210809222454-d867a43fc93e // indirect
	golang.org/x/term v0.0.0-20210317153231-de623e64d2a6 // indirect
	golang.org/x/xerrors v0.0.0-20200804184101-5ec99f83aff1 // indirect
)

// "Fork" of go-sqlite3 which: is updated to SQLite 3.35.4 and removes the
// sqlite_json build constraint so it compiles with JSON support without having
// to specify a build tag.
replace github.com/mattn/go-sqlite3 => github.com/zgoat/go-sqlite3 v1.13.1-0.20210919090901-28392ef30942

// https://github.com/oschwald/maxminddb-golang/pull/75
replace github.com/oschwald/maxminddb-golang => github.com/zgoat/maxminddb-golang v1.8.1-0.20201227124339-dc03187a9664

// https://github.com/oschwald/geoip2-golang/pull/68
replace github.com/oschwald/geoip2-golang => github.com/zgoat/geoip2-golang v1.4.1-0.20201227124715-9eb17ed0da06

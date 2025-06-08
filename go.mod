module zgo.at/goatcounter/v2

go 1.24.3

require (
	code.soquee.net/otp v0.0.4
	github.com/BurntSushi/toml v1.5.0
	github.com/PuerkitoBio/goquery v1.10.3
	github.com/bmatcuk/doublestar/v4 v4.8.1
	github.com/boombuler/barcode v1.0.2
	github.com/go-chi/chi/v5 v5.2.1
	github.com/google/uuid v1.6.0
	github.com/gorilla/websocket v1.5.3
	github.com/mattn/go-sqlite3 v1.14.28
	github.com/monoculum/formam/v3 v3.6.1-0.20221106124510-6a93f49ac1f8
	github.com/oschwald/geoip2-golang v1.11.0
	github.com/oschwald/maxminddb-golang v1.13.1
	github.com/russross/blackfriday/v2 v2.1.0
	github.com/sethvargo/go-limiter v1.0.0
	github.com/teamwork/reload v1.4.2
	golang.org/x/crypto v0.38.0
	golang.org/x/image v0.27.0
	golang.org/x/net v0.40.0
	golang.org/x/sync v0.14.0
	golang.org/x/text v0.25.0
	zgo.at/bgrun v0.0.0-00010101000000-000000000000
	zgo.at/blackmail v0.0.0-20241118175226-9a8f9442bb0d
	zgo.at/errors v1.4.0
	zgo.at/follow v0.0.0-20250412000721-fecf5127a123
	zgo.at/gadget v1.0.0
	zgo.at/guru v1.2.0
	zgo.at/isbot v1.0.0
	zgo.at/jfmt v0.0.0-20240726113937-e6436421fade
	zgo.at/json v0.0.0-20221020004326-fe4f75bb278e
	zgo.at/slog_align v0.0.0-20250608122414-fe848f5abf9c
	zgo.at/termtext v1.5.0
	zgo.at/tz v0.0.0-20240819050900-3c7bf6122612
	zgo.at/z18n v0.0.0-20240522230155-4d5af439f8c4
	zgo.at/zcache v1.2.0
	zgo.at/zcache/v2 v2.1.0
	zgo.at/zdb v0.0.0-20250528164418-859852b74fef
	zgo.at/zhttp v0.0.0-20250606234350-7e8bad41bf7d
	zgo.at/zli v0.0.0-20250601161843-debde58580f1
	zgo.at/zprof v0.0.0-20211217104121-c3c12596d8f0
	zgo.at/zstd v0.0.0-20250313035723-1ece53b5d53e
	zgo.at/ztpl v0.0.0-20250522073924-f73b178f1791
	zgo.at/zvalidate v0.0.0-20250318041547-3c1bd3bca262
)

// Need to finish this and put it in its own repo.
replace zgo.at/bgrun => ./bgrun

// https://github.com/oschwald/maxminddb-golang/pull/75
replace github.com/oschwald/maxminddb-golang => github.com/arp242/maxminddb-golang v1.8.1-0.20221021031716-eb1bbbb3fc5d

// https://github.com/oschwald/geoip2-golang/pull/68
replace github.com/oschwald/geoip2-golang => github.com/arp242/geoip2-golang v1.4.1-0.20220825052315-37df63691c60

require (
	github.com/andybalholm/cascadia v1.3.3 // indirect
	github.com/fsnotify/fsnotify v1.9.0 // indirect
	github.com/lib/pq v1.10.9 // indirect
	github.com/rivo/uniseg v0.4.7 // indirect
	golang.org/x/mod v0.24.0 // indirect
	golang.org/x/sys v0.33.0 // indirect
	golang.org/x/tools v0.33.0 // indirect
	zgo.at/runewidth v0.1.0 // indirect
)

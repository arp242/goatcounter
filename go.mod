module zgo.at/goatcounter/v2

go 1.18

require (
	code.soquee.net/otp v0.0.1
	github.com/BurntSushi/toml v1.1.0
	github.com/PuerkitoBio/goquery v1.8.0
	github.com/bmatcuk/doublestar/v4 v4.0.2
	github.com/boombuler/barcode v1.0.1
	github.com/go-chi/chi/v5 v5.0.7
	github.com/google/uuid v1.3.0
	github.com/gorilla/websocket v1.5.0
	github.com/mattn/go-sqlite3 v1.14.13
	github.com/monoculum/formam v3.5.5+incompatible
	github.com/oschwald/geoip2-golang v1.4.0
	github.com/russross/blackfriday/v2 v2.1.0
	github.com/teamwork/reload v1.4.1
	golang.org/x/crypto v0.0.0-20220525230936-793ad666bf5e
	golang.org/x/image v0.0.0-20220601225756-64ec528b34cd
	golang.org/x/net v0.0.0-20220531201128-c960675eff93
	golang.org/x/sync v0.0.0-20220601150217-0de741cfad7f
	golang.org/x/text v0.3.7
	zgo.at/blackmail v0.0.0-20211212060815-1f8e8a94692b
	zgo.at/errors v1.1.0
	zgo.at/follow v0.0.0-20211017230838-112008350298
	zgo.at/gadget v1.0.0
	zgo.at/guru v1.1.0
	zgo.at/isbot v1.0.0
	zgo.at/json v0.0.0-20211017213340-cc8bf51df08c
	zgo.at/termtext v1.1.0
	zgo.at/tz v0.0.0-20211017223207-006eae99adf6
	zgo.at/z18n v0.0.0-20220606095325-513ddb98b28f
	zgo.at/zcache v1.2.0
	zgo.at/zcache/v2 v2.1.0
	zgo.at/zdb v0.0.0-20220606122939-5f47d7a24eaa
	zgo.at/zhttp v0.0.0-20220306174538-ede1552bdf7c
	zgo.at/zli v0.0.0-20220603144944-094944de19e6
	zgo.at/zlog v0.0.0-20211008102840-46c1167bf2a9
	zgo.at/zprof v0.0.0-20211217104121-c3c12596d8f0
	zgo.at/zstd v0.0.0-20220606095932-df1f8a2ae661
	zgo.at/zstripe v1.1.1-0.20210407063143-62ac9deebc08
	zgo.at/ztpl v0.0.0-20211128061406-6ff34b1256c4
	zgo.at/zvalidate v0.0.0-20211128195927-d13b18611e62
)

require (
	github.com/andybalholm/cascadia v1.3.1 // indirect
	github.com/fsnotify/fsnotify v1.4.9 // indirect
	github.com/lib/pq v1.10.6 // indirect
	github.com/mattn/go-runewidth v0.0.13 // indirect
	github.com/oschwald/maxminddb-golang v1.8.0 // indirect
	github.com/rivo/uniseg v0.2.0 // indirect
	golang.org/x/mod v0.6.0-dev.0.20220419223038-86c51ed26bb4 // indirect
	golang.org/x/sys v0.0.0-20220520151302-bc2c85ada10a // indirect
	golang.org/x/term v0.0.0-20220411215600-e5f449aeb171 // indirect
	golang.org/x/tools v0.1.11-0.20220513221640-090b14e8501f // indirect
)

// https://github.com/oschwald/maxminddb-golang/pull/75
replace github.com/oschwald/maxminddb-golang => github.com/arp242/maxminddb-golang v1.8.1-0.20201227124339-dc03187a9664

// https://github.com/oschwald/geoip2-golang/pull/68
replace github.com/oschwald/geoip2-golang => github.com/arp242/geoip2-golang v1.4.1-0.20201227124715-9eb17ed0da06

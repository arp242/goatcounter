module zgo.at/goatcounter/v2

go 1.19

require (
	code.soquee.net/otp v0.0.4
	github.com/BurntSushi/toml v1.2.0
	github.com/PuerkitoBio/goquery v1.8.0
	github.com/bmatcuk/doublestar/v4 v4.2.0
	github.com/boombuler/barcode v1.0.1
	github.com/go-chi/chi/v5 v5.0.7
	github.com/google/uuid v1.3.0
	github.com/gorilla/websocket v1.5.0
	github.com/mattn/go-sqlite3 v1.14.15
	github.com/monoculum/formam v3.5.5+incompatible
	github.com/oschwald/geoip2-golang v1.4.0
	github.com/russross/blackfriday/v2 v2.1.0
	github.com/teamwork/reload v1.4.1
	golang.org/x/crypto v0.0.0-20220722155217-630584e8d5aa
	golang.org/x/image v0.0.0-20220722155232-062f8c9fd539
	golang.org/x/net v0.0.0-20220811182439-13a9a731de15
	golang.org/x/sync v0.0.0-20220722155255-886fb9371eb4
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
	zgo.at/zdb v0.0.0-20220822042559-7b1209555166
	zgo.at/zhttp v0.0.0-20220306174538-ede1552bdf7c
	zgo.at/zli v0.0.0-20220707072716-d3aefb87935e
	zgo.at/zlog v0.0.0-20211008102840-46c1167bf2a9
	zgo.at/zprof v0.0.0-20211217104121-c3c12596d8f0
	zgo.at/zstd v0.0.0-20220622111728-4a78555db760
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
	golang.org/x/sys v0.0.0-20220728004956-3c1f35247d10 // indirect
	golang.org/x/term v0.0.0-20220411215600-e5f449aeb171 // indirect
	golang.org/x/tools v0.1.11-0.20220513221640-090b14e8501f // indirect
)

// https://github.com/oschwald/maxminddb-golang/pull/75
replace github.com/oschwald/maxminddb-golang => github.com/arp242/maxminddb-golang v1.8.1-0.20201227124339-dc03187a9664

// https://github.com/oschwald/geoip2-golang/pull/68
replace github.com/oschwald/geoip2-golang => github.com/arp242/geoip2-golang v1.4.1-0.20201227124715-9eb17ed0da06

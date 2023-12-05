module zgo.at/goatcounter/v2

go 1.19

require (
	code.soquee.net/otp v0.0.4
	github.com/BurntSushi/toml v1.3.2
	github.com/PuerkitoBio/goquery v1.8.1
	github.com/bmatcuk/doublestar/v4 v4.6.1
	github.com/boombuler/barcode v1.0.1
	github.com/go-chi/chi/v5 v5.0.10
	github.com/google/uuid v1.4.0
	github.com/gorilla/websocket v1.5.1
	github.com/mattn/go-sqlite3 v1.14.18
	github.com/monoculum/formam/v3 v3.6.1-0.20221019142301-7634f9dcc123
	github.com/oschwald/geoip2-golang v1.4.0
	github.com/russross/blackfriday/v2 v2.1.0
	github.com/teamwork/reload v1.4.2
	golang.org/x/crypto v0.16.0
	golang.org/x/exp v0.0.0-20230713183714-613f0c0eb8a1
	golang.org/x/image v0.14.0
	golang.org/x/net v0.19.0
	golang.org/x/sync v0.5.0
	golang.org/x/text v0.14.0
	zgo.at/bgrun v0.0.0-00010101000000-000000000000
	zgo.at/blackmail v0.0.0-20221021025740-b3fdfc32a1aa
	zgo.at/errors v1.2.0
	zgo.at/follow v0.0.0-20221021024812-dd647d64b369
	zgo.at/gadget v1.0.0
	zgo.at/guru v1.1.0
	zgo.at/isbot v1.0.0
	zgo.at/json v0.0.0-20221020004326-fe4f75bb278e
	zgo.at/termtext v1.1.0
	zgo.at/tz v0.0.0-20211017223207-006eae99adf6
	zgo.at/z18n v0.0.0-20221020022658-4ea64eeb51d9
	zgo.at/zcache v1.2.0
	zgo.at/zcache/v2 v2.1.0
	zgo.at/zdb v0.0.0-20230818141326-8a736d26f78a
	zgo.at/zhttp v0.0.0-20230625130145-b6058b7d5c54
	zgo.at/zli v0.0.0-20231124215953-c6675b0b020a
	zgo.at/zlog v0.0.0-20211017235224-dd4772ddf860
	zgo.at/zprof v0.0.0-20211217104121-c3c12596d8f0
	zgo.at/zstd v0.0.0-20230630011541-913635908466
	zgo.at/ztpl v0.0.0-20230614191641-fc02754e9558
	zgo.at/zvalidate v0.0.0-20221021025449-cb54fa8efade
)

// Need to finish this and put it in its own repo.
replace zgo.at/bgrun => ./bgrun

// https://github.com/oschwald/maxminddb-golang/pull/75
replace github.com/oschwald/maxminddb-golang => github.com/arp242/maxminddb-golang v1.8.1-0.20221021031716-eb1bbbb3fc5d

// https://github.com/oschwald/geoip2-golang/pull/68
replace github.com/oschwald/geoip2-golang => github.com/arp242/geoip2-golang v1.4.1-0.20220825052315-37df63691c60

require (
	github.com/andybalholm/cascadia v1.3.1 // indirect
	github.com/fsnotify/fsnotify v1.6.0 // indirect
	github.com/lib/pq v1.10.9 // indirect
	github.com/mattn/go-runewidth v0.0.13 // indirect
	github.com/oschwald/maxminddb-golang v1.10.0 // indirect
	github.com/rivo/uniseg v0.2.0 // indirect
	golang.org/x/mod v0.11.0 // indirect
	golang.org/x/sys v0.15.0 // indirect
	golang.org/x/term v0.15.0 // indirect
	golang.org/x/tools v0.6.0 // indirect
)

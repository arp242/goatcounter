module zgo.at/goatcounter

go 1.13

replace zgo.at/zhttp => ../zhttp

require (
	github.com/arp242/geoip2-golang v1.4.0
	github.com/go-chi/chi v4.0.3+incompatible
	github.com/jmoiron/sqlx v1.2.0
	github.com/lib/pq v1.3.0
	github.com/mattn/go-sqlite3 v2.0.3+incompatible
	github.com/monoculum/formam v0.0.0-20191229172733-952f0766a724
	github.com/mssola/user_agent v0.5.1
	github.com/pkg/errors v0.9.1
	github.com/teamwork/guru v1.0.0
	github.com/teamwork/reload v1.3.0
	golang.org/x/crypto v0.0.0-20200210222208-86ce3cb69678
	zgo.at/tz v0.0.0-20200207054238-b33e5ce779e4
	zgo.at/utils v1.4.1
	zgo.at/zdb v0.0.0-20200210152331-3173dfc581b0
	zgo.at/zhttp v0.0.0-20200213205119-2cbf8136987c
	zgo.at/zlog v1.0.9
	zgo.at/zpack v1.0.1
	zgo.at/zstripe v1.0.0
	zgo.at/ztest v1.0.2
	zgo.at/zvalidate v1.2.2-0.20200212110917-909e6911fd62
)

module zgo.at/goatcounter

go 1.12

require (
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/fsnotify/fsnotify v1.4.7 // indirect
	github.com/getsentry/raven-go v0.2.0
	github.com/go-chi/chi v4.0.2+incompatible
	github.com/go-sql-driver/mysql v1.4.1 // indirect
	github.com/golang/protobuf v1.3.2 // indirect
	github.com/google/go-cmp v0.3.1 // indirect
	github.com/jinzhu/now v1.0.1
	github.com/jmoiron/sqlx v1.2.0
	github.com/lib/pq v1.2.0
	github.com/mattn/go-sqlite3 v1.11.0
	github.com/monoculum/formam v0.0.0-20190830100315-7ff9597b1407 // indirect
	github.com/mssola/user_agent v0.5.0
	github.com/pkg/errors v0.8.1
	github.com/teamwork/guru v0.0.0-20180416195845-617a8909cb7f
	github.com/teamwork/reload v0.0.0-20190319183701-e8d47ccac39e
	github.com/teamwork/test v0.0.0-20190410143529-8897d82f8d46
	github.com/teamwork/utils v0.0.0-20190828152106-44bedcdc1400 // csp branch
	github.com/teamwork/validate v0.0.0-20190828120429-6967b7fc2615
	golang.org/x/crypto v0.0.0-20190829043050-9756ffdc2472 // indirect
	golang.org/x/net v0.0.0-20190827160401-ba9fcec4b297 // indirect
	golang.org/x/sys v0.0.0-20190830080133-08d80c9d36de // indirect
	golang.org/x/tools v0.0.0-20190830082254-f340ed3ae274 // indirect
	google.golang.org/appengine v1.6.2 // indirect
	zgo.at/zhttp v0.0.0-20190827140750-7e240747ece5
	zgo.at/zlog v0.0.0-20190901025635-056660557d15
	zgo.at/zlog_sentry v1.0.0
)

// This fork doesn't depend on the github.com/teamwork/mailaddress package and
// its transient dependencies. Hard to update to upstream due to compatibility.
replace github.com/teamwork/validate => github.com/arp242/validate v0.0.0-20190729142258-60cbc0aff287

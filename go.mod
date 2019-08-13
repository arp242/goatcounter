module zgo.at/goatcounter

go 1.12

//replace zgo.at/zhttp => ../zhttp

// This fork doesn't depend on the github.com/teamwork/mailaddress package and
// its transient dependencies. Hard to update to upstream due to compatibility.
replace github.com/teamwork/validate => github.com/arp242/validate v0.0.0-20190729142258-60cbc0aff287

// csp branch
replace github.com/teamwork/utils => github.com/arp242/utils v0.0.0-20190801113154-2663876b51f4

require (
	github.com/Masterminds/squirrel v1.1.0
	github.com/fsnotify/fsnotify v1.4.7 // indirect
	github.com/getsentry/raven-go v0.2.0
	github.com/go-chi/chi v4.0.2+incompatible
	github.com/go-sql-driver/mysql v1.4.1 // indirect
	github.com/google/go-cmp v0.3.1 // indirect
	github.com/jinzhu/now v1.0.1
	github.com/jmoiron/sqlx v1.2.0
	github.com/lib/pq v1.2.0
	github.com/mattn/go-sqlite3 v1.11.0
	github.com/mssola/user_agent v0.5.0
	github.com/pkg/errors v0.8.1
	github.com/stretchr/testify v1.3.0 // indirect
	github.com/teamwork/guru v0.0.0-20180416195845-617a8909cb7f
	github.com/teamwork/reload v0.0.0-20190319183701-e8d47ccac39e
	github.com/teamwork/test v0.0.0-20190410143529-8897d82f8d46
	github.com/teamwork/utils v0.0.0-20190802180115-518a4b2aa845
	github.com/teamwork/validate v0.0.0-20190729141223-08bcdb8d6ba0
	golang.org/x/crypto v0.0.0-20190701094942-4def268fd1a4 // indirect
	golang.org/x/sys v0.0.0-20190813064441-fde4db37ae7a // indirect
	golang.org/x/tools v0.0.0-20190813034749-528a2984e271 // indirect
	google.golang.org/appengine v1.6.1 // indirect
	zgo.at/zhttp v0.0.0-20190813133347-04e1620b6a0b
	zgo.at/zlog v0.0.0-20190729101808-11a778095e52
	zgo.at/zlog_sentry v1.0.0
)

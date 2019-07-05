module zgo.at/goatcounter

go 1.12

replace zgo.at/zhttp => ../zhttp

replace zgo.at/zlog => ../zlog

replace github.com/monoculum/formam => ../formam

require (
	github.com/Masterminds/squirrel v1.1.0
	github.com/Strum355/go-difflib v1.1.0 // indirect
	github.com/certifi/gocertifi v0.0.0-20190506164543-d2eda7129713 // indirect
	github.com/fsnotify/fsnotify v1.4.7 // indirect
	github.com/getsentry/raven-go v0.2.0
	github.com/go-chi/chi v4.0.2+incompatible
	github.com/go-sql-driver/mysql v1.4.1 // indirect
	github.com/google/go-cmp v0.3.0 // indirect
	github.com/jinzhu/now v1.0.1
	github.com/jmoiron/sqlx v1.2.0
	github.com/lib/pq v1.1.1 // indirect
	github.com/mattn/go-sqlite3 v1.10.0
	github.com/mssola/user_agent v0.5.0
	github.com/pkg/errors v0.8.1
	github.com/stretchr/testify v1.3.0 // indirect
	github.com/teamwork/guru v0.0.0-20180416195845-617a8909cb7f
	github.com/teamwork/reload v0.0.0-20190319183701-e8d47ccac39e
	github.com/teamwork/test v0.0.0-20190410143529-8897d82f8d46
	github.com/teamwork/utils v0.0.0-20190627114848-ce85986393df
	github.com/teamwork/validate v0.0.0-20190704131038-55c9e9c3bcab
	golang.org/x/net v0.0.0-20190628185345-da137c7871d7 // indirect
	golang.org/x/sys v0.0.0-20190626221950-04f50cda93cb // indirect
	google.golang.org/appengine v1.6.1 // indirect
	zgo.at/zhttp v0.0.0-20190705125607-52f33f923714
	zgo.at/zlog v0.0.0-20190618174655-2641de0fc4bf
)

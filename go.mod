module zgo.at/goatcounter

go 1.13

require (
	github.com/go-chi/chi v4.0.2+incompatible
	github.com/jinzhu/now v1.0.1
	github.com/jmoiron/sqlx v1.2.0
	github.com/lib/pq v1.2.0
	github.com/mattn/go-sqlite3 v1.11.0
	github.com/mssola/user_agent v0.5.0
	github.com/pkg/errors v0.8.1
	github.com/teamwork/guru v0.0.0-20180416195845-617a8909cb7f
	github.com/teamwork/reload v0.0.0-20190319183701-e8d47ccac39e
	github.com/teamwork/test v0.0.0-20190410143529-8897d82f8d46
	github.com/teamwork/utils v0.0.0-20190915210805-dd84b3a50562 // mtext branch
	github.com/teamwork/validate v0.0.0-20190828120429-6967b7fc2615
	zgo.at/zdb v0.0.0-20190917160525-a0516a62bb5a
	zgo.at/zhttp v0.0.0-20191014021759-89e9d115dbb7
	zgo.at/zlog v1.0.1
	zgo.at/zpack v0.0.0-20190917143429-094a951f1124
)

// This fork doesn't depend on the github.com/teamwork/mailaddress package and
// its transient dependencies. Hard to update to upstream due to compatibility.
replace github.com/teamwork/validate => github.com/arp242/validate v0.0.0-20190729142258-60cbc0aff287

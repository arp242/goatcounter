// Package cron schedules jobs.
package cron

import (
	"context"
	"time"

	"github.com/Masterminds/squirrel"
	"github.com/jinzhu/now"
	"github.com/jmoiron/sqlx"
	"github.com/teamwork/utils/jsonutil"
	"zgo.at/goatcounter"
	"zgo.at/zhttp/ctxkey"
	"zgo.at/zlog"
)

type task struct {
	fun    func(context.Context) error
	period time.Duration
}

var tasks = []task{
	{goatcounter.Memstore.Persist, 10 * time.Second},
	{updateStats, 2 * time.Hour},
}

// Run stat updates in the background.
func Run(db *sqlx.DB) {
	ctx := context.WithValue(context.Background(), ctxkey.DB, db)
	l := zlog.Module("cron")

	for _, t := range tasks {
		go func(t task) {
			for {
				err := t.fun(ctx)
				if err != nil {
					l.Error(err)
				}
				time.Sleep(t.period)
			}
		}(t)
	}
}

// Wait for all tasks to finish and run all tasks for consistency on shutdown.
func Wait(db *sqlx.DB) {
	ctx := context.WithValue(context.Background(), ctxkey.DB, db)
	l := zlog.Module("cron")

	// TODO(v1): wait for existing.

	for _, t := range tasks {
		err := t.fun(ctx)
		if err != nil {
			l.Error(err)
		}
	}
}

type stat struct {
	Path      string    `db:"path"`
	Count     int       `db:"count"`
	CreatedAt time.Time `db:"created_at"`
}

// TODO(v1): scope to per-site!
//
// TODO(v1): can optimize by not regenerating everything all the time, but adding
//   where created_at >= "2019-06-01"
// and/or split in paths to prevent too much locking
// (already 250ms for just me).
func updateStats(ctx context.Context) error {
	db := goatcounter.MustGetDB(ctx)

	{
		_, err := db.ExecContext(ctx, `delete from hit_stats`)
		if err != nil {
			return err
		}
	}

	l := zlog.Module("stat")
	l.Print("start")

	// Select everything and group by hourly created.
	var stats []stat
	err := db.SelectContext(ctx, &stats, `
		select
			path,
			count(path) as count,
			created_at
		from hits
		group by path, strftime("%Y-%m-%d %H", created_at)
		order by path, strftime("%Y-%m-%d %H", created_at)
	`)
	if err != nil {
		return err
	}

	// {
	//   "jquery.html": map[string][][]int{
	//     "2019-06-22": []{
	// 	     []int{4, 50},
	// 	     []int{6, 4},
	// 	   },
	// 	   "2019-06-23": []{ .. }.
	// 	 },
	// 	 "other.html": { .. },
	// }
	hourly := map[string]map[string][][]int{}
	first := now.BeginningOfDay()
	for _, s := range stats {
		_, ok := hourly[s.Path]
		if !ok {
			hourly[s.Path] = map[string][][]int{}
		}

		if s.CreatedAt.Before(first) {
			first = now.New(s.CreatedAt).BeginningOfDay()
		}

		day := s.CreatedAt.Format("2006-01-02")
		hourly[s.Path][day] = append(hourly[s.Path][day],
			[]int{s.CreatedAt.Hour(), s.Count})
	}

	l.Print("correct")

	// Fill in blank days.
	n := now.BeginningOfDay()
	alldays := []string{first.Format("2006-01-02")}
	for first.Before(n) {
		first = first.Add(24 * time.Hour)
		alldays = append(alldays, first.Format("2006-01-02"))
	}
	allhours := make([][]int, 24)
	for i := 0; i <= 23; i++ {
		allhours[i] = []int{i, 0}
	}
	for path, days := range hourly {
		for _, day := range alldays {
			_, ok := days[day]
			if !ok {
				hourly[path][day] = allhours
			}
		}
	}

	// Fill in blank hours.
	for path, days := range hourly {
		for dayk, day := range days {
			if len(day) == 24 {
				continue
			}

			newday := make([][]int, 24)
		outer:
			for i, hour := range allhours {
				for _, h := range day {
					if h[0] == hour[0] {
						newday[i] = h
						continue outer
					}
				}
				newday[i] = hour
			}

			hourly[path][dayk] = newday
		}
	}

	var have []string
	err = db.SelectContext(ctx, &have, `select path from hit_stats where kind="h"`)
	if err != nil {
		return err
	}

	l.Print("insert")
	squirrel := squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar)
	for path, dayst := range hourly {
		upd := false
		for _, h := range have {
			if h == path {
				upd = true
				break
			}
		}

		sq := squirrel.Insert("hit_stats").Columns("site", "kind", "day", "path", "stats")
		//sq := squirrel.Update("hit_stats")
		//

		for day, st := range dayst {
			var err error
			if upd {
				// TODO(v1)
				// _, err = db.ExecContext(ctx, `update hit_stats
				// 		set stats=$1`, jsonutil.MustMarshal(st))
			} else {
				sq = sq.Values(1, "h", day, path, jsonutil.MustMarshal(st))
			}
			if err != nil {
				return err
			}
		}

		query, args, err := sq.ToSql()
		if err != nil {
			return err
		}

		_, err = db.ExecContext(ctx, query, args...)
	}

	l.Print("done")
	return nil
}

// Package cron schedules jobs.
package cron

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/Masterminds/squirrel"
	"github.com/jinzhu/now"
	"github.com/jmoiron/sqlx"
	"github.com/pkg/errors"
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
	{updateAllStats, 10 * time.Minute},
}

var wg sync.WaitGroup

// Run stat updates in the background.
func Run(db *sqlx.DB) {
	ctx := context.WithValue(context.Background(), ctxkey.DB, db)
	l := zlog.Module("cron")

	for _, t := range tasks {
		go func(t task) {
			defer zlog.Recover()

			for {
				var err error
				func() {
					wg.Add(1)
					defer wg.Done()
					err = t.fun(ctx)
				}()
				if err != nil {
					l.Error(err)
				}
				time.Sleep(t.period)
			}
		}(t)
	}
}

// Wait for all running tasks to finish and then run all tasks for consistency
// on shutdown.
func Wait(db *sqlx.DB) {
	ctx := context.WithValue(context.Background(), ctxkey.DB, db)
	l := zlog.Module("cron")

	wg.Wait()

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

func updateAllStats(ctx context.Context) error {
	var sites goatcounter.Sites
	err := sites.List(ctx)
	if err != nil {
		return err
	}

	for _, s := range sites {
		err := updateSiteStat(ctx, s)
		if err != nil {
			return errors.Wrapf(err, "site %d", s.ID)
		}
	}
	return nil
}

func updateSiteStat(ctx context.Context, site goatcounter.Site) error {
	db := goatcounter.MustGetDB(ctx)
	start := time.Now().Format("2006-01-02")
	l := zlog.Debug("stat").Module("stat")

	// Select everything since last update.
	var last string
	if site.LastStat == nil {
		last = "1970-01-01"
	} else {
		last = site.LastStat.Format("2006-01-02")
	}

	var stats []stat
	err := db.SelectContext(ctx, &stats, `
		select
			path,
			count(path) as count,
			created_at
		from hits
		where
			site=$1 and
			created_at >= $2
		group by path, strftime("%Y-%m-%d %H", created_at)
		order by path, strftime("%Y-%m-%d %H", created_at)
	`, site.ID, last)
	if err != nil {
		fmt.Println("XXX", err)
		return err
	}

	l = l.Since(fmt.Sprintf("fetch from SQL for %d since %s",
		site.ID, last))

	hourly := fillBlanks(stats)
	l = l.Since("Correct data")

	// No data received.
	if len(hourly) == 0 {
		return nil
	}

	// List all paths we already have so we can update them, rather than
	// inserting new.
	var have []string
	err = db.SelectContext(ctx, &have,
		`select path from hit_stats where site=$1 and kind="h"`,
		site.ID)
	if err != nil {
		return err
	}

	insert := squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar).
		Insert("hit_stats").Columns("site", "kind", "day", "path", "stats")
	rows := 0

	// Run insert.
	doInsert := func() error {
		query, args, err := insert.ToSql()
		if err != nil {
			return errors.WithStack(err)
		}
		_, err = db.ExecContext(ctx, query, args...)
		if err != nil {
			return errors.WithStack(err)
		}

		insert = squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar).
			Insert("hit_stats").Columns("site", "kind", "day", "path", "stats")
		rows = 0
		return nil
	}

	for path, dayst := range hourly {
		exists := false
		for _, h := range have {
			if h == path {
				exists = true
				break
			}
		}

		var del []string
		for day, st := range dayst {
			insert = insert.Values(site.ID, "h", day, path, jsonutil.MustMarshal(st))
			if exists {
				del = append(del, `"`+day+`"`)
			}
			rows++
		}

		// Delete existing.
		_, err = db.ExecContext(ctx, `delete from hit_stats where
			site=$1 and path=$2 and day in (`+strings.Join(del, ",")+`)`,
			site.ID, path)
		if err != nil {
			return errors.WithStack(err)
		}

		if rows >= 100 {
			err := doInsert()
			if err != nil {
				return errors.WithStack(err)
			}
		}
	}

	if rows > 0 {
		err = doInsert()
		if err != nil {
			return errors.WithStack(err)
		}
	}

	l = l.Since("Insert in db")

	// Record last update.
	_, err = db.ExecContext(ctx,
		`update sites set last_stat=$1 where id=$2`,
		start, site.ID)
	return errors.WithStack(err)
}

func fillBlanks(stats []stat) map[string]map[string][][]int {
	// Convert data to easier structure:
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

	return hourly
}

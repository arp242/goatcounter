package gomig

import (
	"context"
	"fmt"

	"github.com/jmoiron/sqlx"
	"zgo.at/isbot"
	"zgo.at/zdb"
	"zgo.at/zlog"
)

func Migrate_20200327_1_isbot(db zdb.DB) error {
	zlog.Printf("2020-03-27-1-isbot: this may take a minute, depending on the table size")

	var all []string
	err := db.SelectContext(context.Background(), &all,
		`select browser from hits where bot=0 group by browser`)
	if err != nil {
		return err
	}

	bots := make(map[uint8][]string)
	for _, b := range all {
		bot := isbot.UserAgent(b)
		if isbot.Is(bot) {
			bots[bot] = append(bots[bot], b)
		}
	}

	if len(bots) == 0 {
		return nil
	}

	var total int64
	for bot, ua := range bots {
		query, args, err := sqlx.In(`update hits set bot=? where browser in (?)`, bot, ua)
		if err != nil {
			return err
		}
		query = db.Rebind(query)

		res, err := db.ExecContext(context.Background(), query, args...)
		if err != nil {
			return fmt.Errorf("update hits: %w", err)
		}

		r, _ := res.RowsAffected()
		total += r
	}

	if total > 0 {
		zlog.Printf("2020-03-27-1-isbot: %d hits marked as bot; run 'goatcounter reindex' to update the stats", total)
	}

	return nil
}

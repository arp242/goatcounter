package gomig

import (
	"context"

	"zgo.at/zdb"
)

func ShareAPITokens(ctx context.Context) error {
	var err error
	if zdb.SQLDialect(ctx) == zdb.DialectSQLite {
		err = zdb.Exec(ctx, `alter table api_tokens add column sites varchar not null default '[-1]'`)
	} else {
		err = zdb.Exec(ctx, `alter table api_tokens add column sites jsonb   not null default '[-1]'::jsonb`)
	}
	if err != nil {
		return err
	}

	// Make sure all tokens are on the account ID.
	var tokens []struct {
		APITokenID int64 `db:"api_token_id"`
		Parent     int64 `db:"parent"`
	}
	err = zdb.Select(ctx, &tokens, `
		select api_token_id, parent from api_tokens
		join sites using (site_id)
		where sites.parent is not null
	`)
	if err != nil {
		return err
	}

	for _, t := range tokens {
		err := zdb.Exec(ctx, `update api_tokens set site_id=? where api_token_id=?`, t.Parent, t.APITokenID)
		if err != nil {
			return err
		}
	}
	return nil
}

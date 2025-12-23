package goatcounter

import (
	"context"
	"math/rand/v2"
	"strings"
	"time"

	"zgo.at/errors"
	"zgo.at/goatcounter/v2/pkg/db2"
	"zgo.at/zdb"
	"zgo.at/zstd/ztime"
)

type FilterID int32

type Filter struct {
	FilterID   FilterID  `db:"filter_id"`
	SiteID     SiteID    `db:"site_id"`
	Query      string    `db:"query"`
	CreatedAt  time.Time `db:"created_at"`
	LastUsedAt time.Time `db:"last_used_at"`
}

func (f *Filter) ByQuery(ctx context.Context, query string) error {
	err := zdb.Get(ctx, f, `select * from filters where site_id = ? and lower(query) = lower(?)`,
		MustGetSite(ctx).ID, query)
	if err != nil {
		return errors.Wrapf(err, "Filter.ByQuery(%q)", query)
	}
	err = zdb.Exec(ctx, `update filters set last_used_at = ? where filter_id = ?`, ztime.Now(ctx), f.FilterID)
	return errors.Wrapf(err, "Filter.ByQuery(%q)", query)
}

func (f *Filter) Insert(ctx context.Context, paths []PathID) error {
	f.SiteID = MustGetSite(ctx).ID
	f.CreatedAt = ztime.Now(ctx)
	f.LastUsedAt = ztime.Now(ctx)
	f.FilterID = FilterID(rand.Int32())
	if rand.IntN(2) == 1 {
		f.FilterID = -f.FilterID
	}

	err := zdb.TX(ctx, func(ctx context.Context) error {
		err := zdb.Exec(ctx,
			`insert into filters (filter_id, site_id, query, created_at, last_used_at) values (?, ?, ?, ?, ?)`,
			f.FilterID, MustGetSite(ctx).ID, f.Query, f.CreatedAt, f.LastUsedAt)
		if err != nil {
			return err
		}

		b := zdb.NewBulkInsert(ctx, "filter_paths", []string{"filter_id", "path_id"})
		for _, p := range paths {
			b.Values(f.FilterID, p)
		}
		return b.Finish()
	})
	return errors.Wrap(err, "PathFilter")
}

type PathFilter struct {
	ids      []PathID
	filterID FilterID
}

func (p PathFilter) SQL(ctx context.Context) (zdb.SQL, map[string]any) {
	if p.filterID != 0 {
		return "path_id in (select path_id from filter_paths where filter_id = :filter_id)",
			map[string]any{"filter_id": p.filterID}
	}
	if len(p.ids) == 0 {
		return "1=1", nil
	}
	return "path_id " + db2.In(ctx) + " (:paths)",
		map[string]any{"paths": db2.Array(ctx, p.ids)}
}

func PathFilterFromIDs(ids []PathID) PathFilter {
	return PathFilter{ids: ids}
}

func PathFilterFromQuery(ctx context.Context, query string) (PathFilter, error) {
	like, kw := findFilter(strings.ReplaceAll(query, "%", "%%"),
		"at:start", "at:end", "is:event", "is:pageview", "in:path", "in:title", ":not")
	var (
		onlyEvent, onlyPageview, matchPath, matchTitle, atStart, atEnd bool
		not, or                                                        zdb.SQL
	)
	for _, f := range kw {
		switch f {
		case "at:start":
			atStart = true
		case "at:end":
			atEnd = true
		case "is:event":
			onlyEvent = true
		case "is:pageview":
			onlyPageview = true
		case "in:path":
			matchPath = true
		case "in:title":
			matchTitle = true
		case ":not":
			not = "not"
		}
	}
	if !matchPath && !matchTitle {
		matchPath, matchTitle = true, true
	}
	if matchPath && matchTitle {
		or = "or"
	}
	if !atEnd {
		like = like + "%"
	}
	if !atStart {
		like = "%" + like
	}

	var pathIDs []PathID
	err := zdb.Select(ctx, &pathIDs, "load:paths.PathFilter", map[string]any{
		"site":          MustGetSite(ctx).ID,
		"like":          like,
		"match_title":   matchTitle,
		"match_path":    matchPath,
		"only_event":    onlyEvent,
		"only_pageview": onlyPageview,
		"or":            or,
		"not":           not,
	})
	if err != nil {
		return PathFilter{}, errors.Wrap(err, "PathFilter")
	}

	m := 10_000
	if zdb.SQLDialect(ctx) == zdb.DialectPostgreSQL {
		m = 50_000
	}

	// TODO: be smarter if it filters almost all paths: in that case we can do a
	// "not in ..."
	filter := Filter{Query: query}
	if len(pathIDs) > m {
		err := filter.ByQuery(ctx, query)
		if err != nil && !zdb.ErrNoRows(err) {
			return PathFilter{}, errors.Wrap(err, "PathFilter")
		}
		if zdb.ErrNoRows(err) {
			err := filter.Insert(ctx, pathIDs)
			if err != nil {
				return PathFilter{}, errors.Wrap(err, "PathFilter")
			}
		}
		pathIDs = pathIDs[:0]
	}

	return PathFilter{filterID: filter.FilterID, ids: pathIDs}, nil
}

func findFilter(filter string, find ...string) (string, []string) {
	found := make([]string, 0, 2)
	for _, f := range find {
		if i := strings.Index(filter, f); i > -1 {
			filter = strings.TrimSpace(filter[:i] + filter[i+len(f):])
			found = append(found, f)
		}
	}
	return filter, found
}

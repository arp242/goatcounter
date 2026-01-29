package goatcounter

import (
	"context"
	"math/rand/v2"
	"regexp"
	"strings"
	"time"

	"zgo.at/errors"
	"zgo.at/goatcounter/v2/pkg/db2"
	"zgo.at/zdb"
	"zgo.at/zstd/ztime"
)

type FilterID int32

type Filter struct {
	FilterID   FilterID  `db:"filter_id,readonly"`
	SiteID     SiteID    `db:"site_id,readonly"`
	Matches    int       `db:"matches"`
	Invert     bool      `db:"invert"`
	Query      string    `db:"query"`
	CreatedAt  time.Time `db:"created_at,readonly"`
	LastUsedAt time.Time `db:"last_used_at,readonly"`
}

func (f Filter) Table() string { return "filters" }

var _ zdb.Defaulter = &Filter{}

func (f *Filter) Defaults(ctx context.Context) {
	for f.FilterID == 0 {
		f.FilterID = FilterID(rand.Int32())
		if rand.IntN(2) == 1 {
			f.FilterID = -f.FilterID
		}
	}
	if f.SiteID == 0 {
		f.SiteID = MustGetSite(ctx).ID
	}
	if f.CreatedAt.IsZero() {
		f.CreatedAt = ztime.Now(ctx)
	}
	if f.LastUsedAt.IsZero() {
		f.LastUsedAt = ztime.Now(ctx)
	}
}

var _ zdb.Validator = &APIToken{}

func (f *Filter) Validate(ctx context.Context) error {
	v := NewValidate(ctx)
	return v.ErrorOrNil()
}

func (f *Filter) ByQuery(ctx context.Context, query string) error {
	err := zdb.Get(ctx, f, `select * from filters where site_id=? and lower(query)=lower(?)`,
		MustGetSite(ctx).ID, query)
	return errors.Wrapf(err, "Filter.ByQuery(%q)", query)
}

func (f Filter) Touch(ctx context.Context) error {
	if f.FilterID == 0 {
		return errors.New("Filter.Touch: ID==0")
	}
	err := zdb.Exec(ctx, `update filters set last_used_at=? where filter_id=?`, ztime.Now(ctx), f.FilterID)
	return errors.Wrapf(err, "Filter.Touch(%d)", f.FilterID)
}

func (f *Filter) Insert(ctx context.Context, paths []PathID) error {
	f.Matches = len(paths)
	err := zdb.TX(ctx, func(ctx context.Context) error {
		err := zdb.Insert(ctx, f)
		if err != nil {
			return err
		}

		b, err := zdb.NewBulkInsert(ctx, "filter_paths", []string{"filter_id", "path_id"})
		if err != nil {
			return err
		}
		for _, p := range paths {
			b.Values(f.FilterID, p)
		}
		return b.Finish()
	})
	return errors.Wrap(err, "Filter.Insert")
}

func (f Filter) Append(ctx context.Context, id PathID) error {
	if f.FilterID == 0 {
		return errors.New("Filter.Append: id is 0")
	}
	err := zdb.TX(ctx, func(ctx context.Context) error {
		err := zdb.Exec(ctx, `insert into filter_paths (filter_id, path_id) values (?, ?)`, f.FilterID, id)
		if err != nil {
			return err
		}
		return zdb.Exec(ctx, `update filters set matches = matches + 1 where filter_id = ?`, f.FilterID)
	})
	return errors.Wrap(err, "Filter.Append")
}

func (f Filter) Match(path, title string, event bool) bool {
	like, kw := findFilter(f.Query,
		"at:start", "at:end", "is:event", "is:pageview", "in:path", "in:title", ":not")
	like = strings.ToLower(regexp.QuoteMeta(like))
	var matchPath, matchTitle, not bool
	for _, f := range kw {
		switch f {
		case "at:start":
			like = "^" + like
		case "at:end":
			like = like + "$"
		case "is:event":
			if !event {
				return false
			}
		case "is:pageview":
			if event {
				return false
			}
		case "in:path":
			matchPath = true
		case "in:title":
			matchTitle = true
		case ":not":
			not = true
		}
	}
	if !matchPath && !matchTitle {
		matchPath, matchTitle = true, true
	}

	re, err := regexp.Compile(like)
	if err != nil {
		return false
	}
	var match bool
	if matchPath {
		match = re.MatchString(path)
	}
	if matchTitle && !match {
		match = re.MatchString(title)
	}
	if not {
		match = !match
	}
	return match
}

type Filters []Filter

func (f *Filters) List(ctx context.Context) error {
	err := zdb.Select(ctx, f, `select * from filters where site_id=?`, MustGetSite(ctx).ID)
	return errors.Wrap(err, "Filters.List")
}

type PathFilter struct {
	ids      []PathID
	filterID FilterID
	invert   bool
}

func (p PathFilter) SQL(ctx context.Context) (zdb.SQL, map[string]any) {
	if p.filterID != 0 {
		if p.invert {
			return "path_id not in (select path_id from filter_paths where filter_id = :filter_id)",
				map[string]any{"filter_id": p.filterID}
		}
		return "path_id in (select path_id from filter_paths where filter_id = :filter_id)",
			map[string]any{"filter_id": p.filterID}
	}
	if len(p.ids) == 0 {
		return "1=1", nil
	}
	if p.invert {
		return db2.NotIn(ctx, "path_id") + " (:paths)",
			map[string]any{"paths": db2.Array(ctx, p.ids)}
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
	if like == "" {
		matchPath, matchTitle = false, false
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

	getPathIDs := func(scan any, invert bool) error {
		return zdb.Select(ctx, scan, "load:paths.PathFilter", map[string]any{
			"site":          MustGetSite(ctx).ID,
			"like":          like,
			"match_title":   matchTitle,
			"match_path":    matchPath,
			"have_like":     matchTitle || matchPath,
			"only_event":    onlyEvent,
			"only_pageview": onlyPageview,
			"or":            or,
			"not":           not,
			"invert":        invert,
		})
	}

	var pathIDs []PathID
	err := getPathIDs(&pathIDs, false)
	if err != nil {
		return PathFilter{}, errors.Wrap(err, "PathFilter")
	}

	var invert bool
	// If there's tons of matches then check if we can invert the match. "List
	// all except these 10,000" is lots faster than "include these 500,000
	// paths".
	//
	// TODO: we always want to use a join in these cases, regardless of how long
	// paths is, I think? Need to check, but otherwise it will always add these
	// two queries.
	if len(pathIDs) > 100_000 {
		var invertIDs []PathID
		err := getPathIDs(&invertIDs, true)
		if err != nil {
			return PathFilter{}, errors.Wrap(err, "PathFilter")
		}
		if len(pathIDs) > len(invertIDs) {
			pathIDs, invert = invertIDs, true
		}
	}

	// For SQLite we want to use a join as it limits the parameters to 32k. In
	// hit_list.GetTotalCount it uses the path lists three times, so more than
	// ~10k paths will error out.
	//
	// For PostgreSQL we can set the limit higher as it uses a single array
	// parameter. When using very large arrays PostgreSQL will spend all its
	// time in deconstruct_array().
	m := 10_000
	if zdb.SQLDialect(ctx) == zdb.DialectPostgreSQL {
		m = 50_000
	}
	filter := Filter{Query: query, Invert: invert}
	if len(pathIDs) > m {
		err := zdb.TX(ctx, func(ctx context.Context) error {
			err := filter.ByQuery(ctx, query)
			if err != nil {
				if zdb.ErrNoRows(err) {
					return filter.Insert(ctx, pathIDs)
				}
				return err
			}
			return filter.Touch(ctx)
		})
		if err != nil {
			return PathFilter{}, errors.Wrap(err, "PathFilter")
		}
		pathIDs = pathIDs[:0]
	}

	return PathFilter{filterID: filter.FilterID, ids: pathIDs, invert: invert}, nil
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

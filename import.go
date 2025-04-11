package goatcounter

import (
	"context"
	"encoding/csv"
	"io"
	"net/url"
	"strconv"
	"strings"

	"zgo.at/errors"
	"zgo.at/guru"
	"zgo.at/zdb"
)

func ImportGA(ctx context.Context, fp io.Reader) error {
	var (
		r      = csv.NewReader(fp)
		lineno int
		pages  = make(map[string]int)
	)
	r.Comment = '#'
	for {
		lineno++
		row, err := r.Read()
		if lineno == 1 { // Skip header.
			continue
		}
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return guru.Errorf(400, "line %d: %w", lineno, err)
		}
		if len(row) < 3 {
			return guru.Errorf(400, "line %d: not enough columns (need at least 3)", lineno)
		}

		page, views := strings.TrimSpace(row[0]), strings.TrimSpace(row[2])
		if page == "" {
			continue
		}

		n, err := strconv.Atoi(views)
		if err != nil {
			return guru.Errorf(400, "line %d: %w", lineno, err)
		}

		// Some pages appear as escaped, some don't, because reasons.
		p, err := url.PathUnescape(page)
		if err == nil {
			page = p
		}
		page = "/" + strings.Trim(page, "/")
		pages[page] += n
	}

	var (
		siteID = MustGetSite(ctx).ID
		paths  = make(map[string]int64)
	)
	for p := range pages {
		pp := Path{
			Site: siteID,
			Path: p,
		}
		err := pp.GetOrInsert(ctx)
		if err != nil {
			return err
		}
		paths[p] = pp.ID
	}

	ins := zdb.NewBulkInsert(ctx, "hit_counts", []string{"site_id", "path_id", "hour", "total"})
	if zdb.SQLDialect(ctx) == zdb.DialectPostgreSQL {
		ins.OnConflict(`on conflict on constraint "hit_counts#site_id#path_id#hour" do update set
			total = hit_counts.total + excluded.total`)
	} else {
		ins.OnConflict(`on conflict(site_id, path_id, hour) do update set
			total = hit_counts.total + excluded.total`)
	}
	for k, v := range pages {
		ins.Values(siteID, paths[k], "1980-01-01 00:00:00", v)
	}
	return ins.Finish()
}

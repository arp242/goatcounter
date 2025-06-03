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
		r         = csv.NewReader(fp)
		i         int
		pages     = make(map[string]int)
		pageMatch = make(map[string]string)
	)
	r.Comment = '#'
	for {
		i++
		row, err := r.Read()
		line0, col0 := r.FieldPos(0)
		line2, col2 := r.FieldPos(2)
		if i == 1 { // Header.
			if h := "Page path and screen class"; row[0] != h {
				return guru.Errorf(400, "line %d:%d: first header is %q, but expecting it to be %q",
					line0, col0, row[0], h)
			}
			if h := "Active users"; row[2] != h {
				return guru.Errorf(400, "line %d:%d: third header is %q, but expecting it to be %q",
					line2, col2, row[2], h)
			}
			continue
		}
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return guru.Errorf(400, "line %d: %w", line0, err)
		}
		if len(row) < 3 {
			return guru.Errorf(400, "line %d: not enough columns (need at least 3)", line0)
		}

		page, views := strings.TrimSpace(row[0]), strings.TrimSpace(row[2])
		if page == "" {
			continue
		}

		n, err := strconv.Atoi(views)
		if err != nil {
			return guru.Errorf(400, "line %d:%d: %q columns: %w", line2, col2, "Views per active user", err)
		}

		// Some pages appear as escaped, some don't, because reasons.
		p, err := url.PathUnescape(page)
		if err == nil {
			page = p
		}
		page = "/" + strings.Trim(page, "/")

		// Match case-insensitive, but keep the first casing we see.
		l := strings.ToLower(page)
		m, ok := pageMatch[l]
		if ok {
			page = m
		} else {
			pageMatch[l] = page
		}
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

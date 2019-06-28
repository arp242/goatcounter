package goatcounter

import (
	"context"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/teamwork/utils/jsonutil"
	"github.com/teamwork/validate"
)

// Normalize:
//
// - Group by domain, and related domains.
// - Then display specific "deep links" as sublevels

// var groups = map[string]string{
// 	"https://news.ycombinator.com": "Hacker News",
// 	"https://hn.algolia.com":       "Hacker News",
// 	"http://hckrnews.com":          "Hacker News",
// 	"android-app://com.stefandekanski.hackernews.free": "Hacker News",
//
// 	"https://en.m.wikipedia.org": "Wikipedia",
// 	"https://en.wikipedia.org": "Wikipedia",
//
// 	"https://www.google.*": "Google",
//
//  // https://twitter.com/search?lang=en&q=%22https%3A%2F%2Ft.co%2FCZIy0OlYQn%22&src=typed_query
// 	"https://t.co/*": "Twitter",
// 	"android-app://com.twitpane": "Twitter",
//
// 	"https://www.reddit.com/*": "Reddit",
// 	"https://old.reddit.com/*": "Reddit",
// 	"android-app://com.laurencedawson.reddit_sync": "Reddit",
// 	"android-app://com.laurencedawson.reddit_sync.pro": "Reddit",
// 	"android-app://com.laurencedawson.reddit_sync.dev": "Reddit",
// 	"android-app://com.andrewshu.android.reddit": "Reddit",
//
// 	"https://mail.google.com/mail/u/0": "Gmail",
// 	"android-app://com.google.android.gm": "Gmail",
//
// 	android-app://org.telegram.messenger
// 	android-app://io.github.hidroh.materialistic
// 	android-app://com.Slack
// 	android-app://com.linkedin.android
// 	android-app://m.facebook.com
// 	android-app://com.noinnion.android.greader.reader
//
// }

type Hit struct {
	Site int64 `db:"site" json:"-"`

	Path      string    `db:"path" json:"p,omitempty"`
	Ref       string    `db:"ref" json:"r,omitempty"`
	RefParams *string   `db:"ref_params" json:"ref_params,omitempty"`
	CreatedAt time.Time `db:"created_at" json:"-"`
}

// Defaults sets fields to default values, unless they're already set.
func (h *Hit) Defaults(ctx context.Context) {
	site := MustGetSite(ctx)
	h.Site = site.ID

	if h.CreatedAt.IsZero() {
		h.CreatedAt = time.Now().UTC()
	}

	if h.Ref != "" {
		i := strings.Index(h.Ref, "?")
		if i > 0 {
			rp := h.Ref[i+1:]
			h.RefParams = &rp
			h.Ref = h.Ref[:i]
		}
	}

	h.Ref = strings.TrimRight(h.Ref, "/")
	h.Path = "/" + strings.Trim(h.Path, "/")
}

// Validate the object.
func (h *Hit) Validate(ctx context.Context) error {
	v := validate.New()

	v.Required("site", h.Site)
	v.Required("path", h.Path)

	return v.ErrorOrNil()
}

// Insert a new row.
func (h *Hit) Insert(ctx context.Context) error {
	db := MustGetDB(ctx)
	h.Defaults(ctx)
	err := h.Validate(ctx)
	if err != nil {
		return err
	}

	_, err = db.ExecContext(ctx, `insert into hits (site, path, ref, ref_params)
		values ($1, $2, $3, $4)`, h.Site, h.Path, h.Ref, h.RefParams)
	return errors.Wrap(err, "Site.Insert")
}

type HitStats []struct {
	Count int    `db:"count"`
	Max   int    `db:"-"`
	Path  string `db:"path"`
	Stats map[string][][]int
}

func (h *HitStats) List(ctx context.Context, start, end time.Time) error {
	db := MustGetDB(ctx)
	site := MustGetSite(ctx)
	err := db.SelectContext(ctx, h, `
		select path, count(path) as count
		from hits
		where
			site=$1 and
			date(created_at) >= $2 and
			date(created_at) <= $3
		group by path
		order by count desc
		limit 500`, site.ID, start.Format("2006-01-02"), end.Format("2006-01-02"))
	if err != nil {
		return errors.Wrap(err, "HitStats.List")
	}

	// Add stats
	type stats struct {
		Path  string    `json:"path"`
		Day   time.Time `json:"day"`
		Stats []byte    `json:"stats"`
	}
	var st []stats
	err = db.SelectContext(ctx, &st, `
		select path, day, stats
		from hit_stats
		where
			site=$1 and
			kind="h" and
			date(day) >= $2 and
			date(day) <= $3
		`, site.ID, start.Format("2006-01-02"), end.Format("2006-01-02"))
	if err != nil {
		return errors.Wrap(err, "HitStats.List")
	}

	// TODO: meh...
	hh := *h
	for i := range hh {
		hh[i].Stats = map[string][][]int{}

		for _, s := range st { // []stats
			if s.Path == hh[i].Path {
				var x [][]int
				jsonutil.MustUnmarshal(s.Stats, &x)
				hh[i].Stats[s.Day.Format("2006-01-02")] = x

				// Get max.
				// TODO: should maybe store this?
				for j := range x {
					if x[j][1] > hh[i].Max {
						hh[i].Max = x[j][1]
					}
				}
			}
		}

		if hh[i].Max < 10 {
			hh[i].Max = 10
		}
	}

	return nil
}

// ListRefs lists all references for a path.
func (h *HitStats) ListRefs(ctx context.Context, path string, start, end time.Time) error {
	db := MustGetDB(ctx)
	site := MustGetSite(ctx)

	err := db.SelectContext(ctx, h, `
		select ref as path, count(ref) as count
		from hits
		where
			site=$1 and
			path=$2S and
			date(created_at) >= $3 and
			date(created_at) <= $4
		group by ref
		order by count(*) desc
		limit 50
	`, site.ID, path, start.Format("2006-01-02"), end.Format("2006-01-02"))

	return errors.Wrap(err, "RefStats.ListRefs")
}

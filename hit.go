package goatcounter

import (
	"context"
	"net/url"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/teamwork/utils/jsonutil"
	"github.com/teamwork/validate"
	"zgo.at/zlog"
)

type Hit struct {
	Site int64 `db:"site" json:"-"`

	Path        string    `db:"path" json:"p,omitempty"`
	Ref         string    `db:"ref" json:"r,omitempty"`
	RefParams   *string   `db:"ref_params" json:"ref_params,omitempty"`
	RefOriginal *string   `db:"ref_url" json:"ref_params,omitempty"`
	CreatedAt   time.Time `db:"created_at" json:"-"`

	refURL *url.URL `db:"-" json:"-"`
}

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

// TODO:
// If the reference is "hn.algolia.com" then set it to "news.ycombinator.com",
// and set the RefOriginal to "hn.algolia.com".
//
// 	"https://news.ycombinator.com": "Hacker News",
// 	"https://hn.algolia.com":       "Hacker News",
//
func cleanURL(ref string, refURL *url.URL) (string, *string) {
	i := strings.Index(ref, "?")
	if i == -1 {
		return ref, nil
	}

	eq := ref[i+1:]
	ref = ref[:i]

	// Ignore params for gmail; it's never useful.
	if refURL.Host == "mail.google.com" {
		return ref, nil
	}
	// Twitter's t.co links add this.
	if eq == "amp=1" {
		return ref, nil
	}

	q := refURL.Query()

	// Google analytics tracking parameters.
	q.Del("utm_source")
	q.Del("utm_medium")
	q.Del("utm_campaign")
	q.Del("utm_term")

	// ref = https://getpocket.com/redirect
	// ref_params = url=https%3A%2F%2Fjavascriptweekly.com%2Flink%2F64733%2F2609d695ae&h=3aafa9fcffe6a8536ed5998028273ebc34ad670c2328e66948e90de570ca0224

	// ref = https://www.google.se/url
	// ref_params = sa=t&rct=j&q=&esrc=s&source=web&cd=3&ved=2ahUKEwi76qzW0_LiAhWu1aYKHbqKB70QFjACegQIAxAB&url=https%3A%2F%2Farp242.net%2Fphp-fopen-is-broken.html&usg=AOvVaw0OUYrWh-k8Suse9hHDfdeW

	if len(q) == 0 {
		return ref, nil
	}

	eq = q.Encode()
	return ref, &eq
}

// Defaults sets fields to default values, unless they're already set.
func (h *Hit) Defaults(ctx context.Context) {
	// site := MustGetSite(ctx)
	// h.Site = site.ID

	if h.CreatedAt.IsZero() {
		h.CreatedAt = time.Now().UTC()
	}

	if h.Ref != "" && h.refURL != nil {
		h.Ref, h.RefParams = cleanURL(h.Ref, h.refURL)
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
	var err error
	h.refURL, err = url.Parse(h.Ref)
	if err != nil {
		zlog.Fields(zlog.F{"ref": h.Ref}).Errorf("could not parse ref: %s", err)
	}

	// Ignore spammers.
	if _, ok := blacklist[h.refURL.Host]; ok {
		return nil
	}

	h.Defaults(ctx)
	err = h.Validate(ctx)
	if err != nil {
		return err
	}

	// TODO: don't insert right away, cache in memory for 5s or so and then insert.
	// Memstore.Lock()
	// Memstore = append(Memstore, ...)
	// Memstore.Unlock()

	db := MustGetDB(ctx)
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

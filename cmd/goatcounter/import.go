// Copyright © 2019 Martin Tournoij <martin@arp242.net>
// This file is part of GoatCounter and published under the terms of the EUPL
// v1.2, which can be found in the LICENSE file or at http://eupl12.zgo.at

package main

import (
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"zgo.at/errors"
	"zgo.at/goatcounter"
	"zgo.at/goatcounter/cfg"
	"zgo.at/goatcounter/handlers"
	"zgo.at/json"
	"zgo.at/zdb"
	"zgo.at/zli"
	"zgo.at/zlog"
	"zgo.at/zstd/zint"
	"zgo.at/zstd/zstring"
)

const usageImport = `
Import pageviews from an export

You must give one filename to import; use - to read from stdin:

    $ goatcounter import export.csv.gz

This requires a running GoatCounter instance; it's a front-end for the API
rather than a tool to modify the database directly. If you're running this on
the same machine the data will be fetched from the DB and a temporary API key
will be created.

Or use an URL in -site if you want to send data to another instance:

    $ export API_KEY=[..]
    $ goatcounter import -site https://stats.example.com

Flags:

  -db          Database connection: "sqlite://<file>" or "postgres://<connect>"
               See "goatcounter help db" for detailed documentation. Default:
               sqlite://db/goatcounter.sqlite3?_busy_timeout=200&_journal_mode=wal&cache=shared

  -debug       Modules to debug, comma-separated or 'all' for all modules.

  -silent      Don't show progress information.

  -site        Site to import to, not needed if there is only one site, as an ID
               ("1"), code ("example"), or an URL ("https://stats.example.com").
               You must set GOATCOUNTER_API_KEY if you use an URL.

  -format      File format; currently accepted values:

                   csv   GoatCounter CSV export (default)

Environment:

  GOATCOUNTER_API_KEY   API key to use if you're connecting to a remote API;
                        must have "count" permission.
`

var silent bool

func importCmd() (int, error) {
	// So it uses https URLs in site.URL()
	// TODO: should fix it to always use https even on dev and get rid of the
	// exceptions.
	cfg.Prod = true

	dbConnect := flagDB()
	debug := flagDebug()

	var format, siteFlag string
	CommandLine.StringVar(&siteFlag, "site", "", "")
	CommandLine.StringVar(&format, "format", "csv", "")
	CommandLine.BoolVar(&silent, "silent", false, "")
	err := CommandLine.Parse(os.Args[2:])
	if err != nil {
		return 1, err
	}

	files := CommandLine.Args()
	if len(files) == 0 {
		return 1, fmt.Errorf("need a filename")
	}
	if len(files) > 1 {
		return 1, fmt.Errorf("can only specify one filename")
	}

	var fp io.ReadCloser
	if files[0] == "-" {
		fp = ioutil.NopCloser(os.Stdin)
	} else {
		fp, err = os.Open(files[0])
		if err != nil {
			return 1, err
		}
		defer fp.Close()
	}

	zlog.Config.SetDebug(*debug)

	url, key, clean, err := findSite(siteFlag, *dbConnect)
	if err != nil {
		return 1, err
	}
	if clean != nil {
		defer clean()
	}

	var n int
	switch format {
	default:
		return 1, fmt.Errorf("unknown -format value: %q", format)
	case "csv":
		n, err = importCSV(fp, url, key)
	}
	if err != nil {
		var gErr *errors.Group
		if errors.As(err, &gErr) {
			return 1, fmt.Errorf("%d errors", gErr.Len())
		}
		return 1, err
	}

	if !silent {
		zli.EraseLine()
		fmt.Printf("Done! Imported %d rows\n", n)
	}
	return 0, nil
}

var (
	importClient = http.Client{Timeout: 5 * time.Second}
	nSent        int
)

func newRequest(method, url, key string, body io.Reader) (*http.Request, error) {
	r, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, err
	}
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Authorization", "Bearer "+key)
	return r, nil
}

func importSend(url, key string, hits []handlers.APICountRequestHit) error {
	body, err := json.Marshal(handlers.APICountRequest{Hits: hits})
	if err != nil {
		return err
	}

	if !silent {
		zli.ReplaceLinef("Sending %d hits to %s…", len(hits), url)
	}

	r, err := newRequest("POST", url, key, bytes.NewReader(body))
	if err != nil {
		return err
	}

	resp, err := importClient.Do(r)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	// All okay!
	case 202:
		if !silent {
			fmt.Print(" Okay!")
		}

	// Rate limit
	case 429:
		s, err := strconv.Atoi(resp.Header.Get("X-Rate-Limit-Reset"))
		if err != nil {
			return err
		}

		if !silent {
			zli.ReplaceLinef("Rate limited; sleeping for %d seconds\n", s)
		}
		time.Sleep(time.Duration(s) * time.Second)

	// Other error
	default:
		b, _ := ioutil.ReadAll(resp.Body)
		return fmt.Errorf("%s: %s: %s", url, resp.Status, zstring.ElideLeft(string(b), 200))
	}

	nSent += len(hits)

	// Sleep until next Memstore cycle.
	if nSent > 2000 {
		last, err := time.Parse(time.RFC3339Nano, resp.Header.Get("X-Goatcounter-Memstore"))
		if err != nil {
			return err
		}
		s := time.Until(last.Add(10_000 * time.Millisecond))

		if !silent {
			zli.ReplaceLinef("Sent 2,000 pageviews; Waiting %s", s.Round(time.Millisecond))
		}
		time.Sleep(s)

		nSent = 0
	}
	return nil
}

func importCSV(fp io.Reader, url, key string) (int, error) {
	c := csv.NewReader(fp)
	header, err := c.Read()
	if err != nil {
		return 0, err
	}
	if len(header) == 0 || !strings.HasPrefix(header[0], goatcounter.ExportVersion) {
		return 0, errors.Errorf(
			"wrong version of CSV database: %s (expected: %s)",
			header[0][:1], goatcounter.ExportVersion)
	}

	var (
		n        = 0
		sessions = make(map[zint.Uint128]zint.Uint128)
		errs     = errors.NewGroup(50)
		hits     = make([]handlers.APICountRequestHit, 0, 100)
	)
	for {
		line, err := c.Read()
		if err == io.EOF {
			break
		}
		if errs.Append(err) {
			if !silent {
				zli.Errorf(err)
			}
			continue
		}

		var row goatcounter.ExportRow
		if errs.Append(row.Read(line)) {
			if !silent {
				zli.Errorf(err)
			}
			continue
		}

		hit, err := row.Hit(0)
		if errs.Append(row.Read(line)) {
			if !silent {
				zli.Errorf(err)
			}
			continue
		}

		// Map session IDs to new session IDs.
		s, ok := sessions[hit.Session]
		if !ok {
			sessions[hit.Session] = goatcounter.Memstore.SessionID()
		}
		hit.Session = s

		hits = append(hits, handlers.APICountRequestHit{
			Path:      hit.Path,
			Title:     hit.Title,
			Event:     hit.Event,
			Ref:       hit.Ref,
			Size:      hit.Size,
			Bot:       hit.Bot,
			UserAgent: hit.Browser,
			Location:  hit.Location,
			CreatedAt: hit.CreatedAt,
			Session:   hit.Session.String(),
		})
		if len(hits) >= 100 {
			if errs.Append(importSend(url, key, hits)) {
				if !silent {
					zli.Errorf(err)
				}
			}
			hits = make([]handlers.APICountRequestHit, 0, 100)
		}
		n++
	}
	if len(hits) > 0 {
		errs.Append(importSend(url, key, hits))
	}

	return n, errs.ErrorOrNil()
}

func findSite(siteFlag, dbConnect string) (string, string, func(), error) {
	var (
		url, key string
		clean    func()
	)
	switch {
	case strings.HasPrefix(siteFlag, "http://") || strings.HasPrefix(siteFlag, "https://"):
		url = strings.TrimRight(siteFlag, "/")
		url = strings.TrimSuffix(url, "/api/v0/count")
		if !strings.HasPrefix(url, "http") {
			url = "https://" + url
		}

		key = os.Getenv("GOATCOUNTER_API_KEY")
		if key == "" {
			return "", "", nil, errors.New("GOATCOUNTER_API_KEY must be set")
		}

	default:
		db, err := connectDB(dbConnect, nil, false)
		if err != nil {
			return "", "", nil, err
		}
		defer db.Close()
		ctx := zdb.With(context.Background(), db)

		var site goatcounter.Site
		siteID, intErr := strconv.ParseInt(siteFlag, 10, 64)
		switch {
		default:
			err = site.ByCode(ctx, siteFlag)
		case intErr != nil && siteID > 0:
			err = site.ByID(ctx, siteID)
		case siteFlag == "":
			var sites goatcounter.Sites
			err := sites.UnscopedList(ctx)
			if err != nil {
				return "", "", nil, err
			}

			switch len(sites) {
			case 0:
				return "", "", nil, fmt.Errorf("there are no sites in the database")
			case 1:
				site = sites[0]
			default:
				return "", "", nil, fmt.Errorf("more than one site: use -site to specify which site to import")
			}
		}
		if err != nil {
			return "", "", nil, err
		}
		ctx = goatcounter.WithSite(ctx, &site)

		var user goatcounter.User
		err = user.BySite(ctx, site.ID)
		if err != nil {
			return "", "", nil, err
		}
		ctx = goatcounter.WithUser(ctx, &user)

		token := goatcounter.APIToken{
			SiteID:      site.ID,
			Name:        "goatcounter import",
			Permissions: goatcounter.APITokenPermissions{Count: true},
		}
		err = token.Insert(ctx)
		if err != nil {
			return "", "", nil, err
		}

		url = site.URL() + "/api/v0/count"
		key = token.Token
		clean = func() { token.Delete(ctx) }
	}

	// Verify that the site is live and that we've got the correct permissions.
	r, err := newRequest("GET", url+"/api/v0/me", key, nil)
	if err != nil {
		return "", "", nil, err
	}

	resp, err := importClient.Do(r)
	if err != nil {
		return "", "", nil, err
	}
	defer resp.Body.Close()
	b, _ := ioutil.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return "", "", nil, fmt.Errorf("%s: %s: %s", url+"/api/v0/me",
			resp.Status, zstring.ElideLeft(string(b), 200))
	}

	var perm struct {
		Token goatcounter.APIToken `json:"token"`
	}
	err = json.Unmarshal(b, &perm)
	if err != nil {
		return "", "", nil, err
	}
	if !perm.Token.Permissions.Count {
		return "", "", nil, fmt.Errorf("the API toke %q is missing the 'count' permission", perm.Token.Name)
	}

	return url + "/api/v0/count", key, clean, nil
}

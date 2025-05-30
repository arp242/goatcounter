package main

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"zgo.at/errors"
	"zgo.at/goatcounter/v2"
	"zgo.at/termtext"
	"zgo.at/zli"
	"zgo.at/zstd/zint"
	"zgo.at/zstd/zstring"
	"zgo.at/zstd/ztime"
)

const usageDashboard = `
View the dashboard of a GoatCounter installation via the API.

This requires an API key with the "read sites" and "read statistics"
permissions.

This is fairly simple for the time being.

Environment:

  GOATCOUNTER_API_KEY   API key; requires "read sites" and "read statistics"
                        permissions.

Flags:

  -site        Site to user, as an URL (e.g. "https://stats.example.com")

  -range       Time range to show; defaults to last 7 days. Formats:

                   <any number>              Last n days
                   "2022-01-01:2022-01-31"   Explicit start and end date

  Day to show, as year-month-day; default is the current day.
`

var dashClient = http.Client{Timeout: 15 * time.Second}

// Load the dashboard in the terminal with the API.
func cmdDashboard(f zli.Flags) error {
	if len(f.Args) == 0 {
		printHelp(usage["dashboard"])
		return nil
	}

	// Parse commandline arguments.
	var (
		site      = f.String("", "site")
		rangeFlag = f.String("", "range")
	)
	if err := f.Parse(zli.FromEnv("GOATCOUNTER")); err != nil {
		return err
	}

	if site.String() == "" {
		return errors.New("-site must be set")
	}

	rng, err := parseRange(rangeFlag.String())
	if err != nil {
		return err
	}

	url := strings.TrimRight(site.String(), "/")
	if !zstring.HasPrefixes(url, "http://", "https://") {
		url = "https://" + url
	}

	// Get API key and ensure we have the correct permissions.
	key := os.Getenv("GOATCOUNTER_API_KEY")
	if key == "" {
		return errors.New("GOATCOUNTER_API_KEY must be set")
	}
	err = checkSite(url, key, goatcounter.APIPermSiteRead, goatcounter.APIPermStats)
	if err != nil {
		return err
	}

	return dash(url, key, rng)
}

// Parse -range flag.
func parseRange(rangeFlag string) (ztime.Range, error) {
	var (
		now = ztime.Time{ztime.Now()}
		// Default to last week.
		rng = ztime.NewRange(now.AddPeriod(-7, ztime.Day).Time).To(now.Time)
	)

	// No flag: return default.
	if rangeFlag == "" {
		return rng, nil
	}

	// -range 30 for last 30 days.
	n, err := strconv.ParseInt(rangeFlag, 0, 64)
	if err == nil {
		rng.Start = now.AddPeriod(int(-n), ztime.Day).Time
		return rng, nil
	}

	// Parse explicit dates as "2006-01-02:2006-01-02"
	startFlag, endFlag, ok := strings.Cut(rangeFlag, ":")
	if !ok {
		return rng, fmt.Errorf("unknown format for -range: %q", rangeFlag)
	}

	rng.Start, err = time.Parse("2006-01-02", startFlag)
	if err != nil {
		return rng, fmt.Errorf("unknown format for -range: %q", rangeFlag)
	}
	rng.End, err = time.Parse("2006-01-02", endFlag)
	if err != nil {
		return rng, fmt.Errorf("unknown format for -range: %q", rangeFlag)
	}
	rng.End = ztime.EndOf(rng.End, ztime.Day)

	return rng, nil
}

// Create a new request.
func newRequest(method, url, key string, body io.Reader) (*http.Request, error) {
	r, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, err
	}
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Authorization", "Bearer "+key)
	return r, nil
}

// Verify that the site is live and that we've got the correct permissions.
func checkSite(url, key string, perms ...zint.Bitflag64) error {
	r, err := newRequest("GET", url+"/api/v0/me", key, nil)
	if err != nil {
		return err
	}

	resp, err := importClient.Do(r)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 429 { // Ratelimit, try again.
		return checkSite(url, key, perms...)
	}

	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return fmt.Errorf("%s: %s: %s", url+"/api/v0/me",
			resp.Status, zstring.ElideLeft(string(b), 200))
	}

	var perm struct {
		Token goatcounter.APIToken `json:"token"`
	}
	err = json.Unmarshal(b, &perm)
	if err != nil {
		return err
	}
	for _, p := range perms {
		if !perm.Token.Permissions.Has(p) {
			// TODO: better error here; include the permission we need.
			return fmt.Errorf("API token %q is missing the required permissions", perm.Token.Name)
		}
	}

	return nil
}

// Make a HTTP request, taking rate limiting in to account.
func doRequest(scanTo any, key string, url string, urlFmt ...any) error {
	r, err := newRequest("GET", fmt.Sprintf(url, urlFmt...), key, nil)
	if err != nil {
		return err
	}

	resp, err := dashClient.Do(r)
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	if resp.StatusCode == 429 { // Ratelimit
		return doRequest(scanTo, key, url, urlFmt...)
	}

	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("status %s: %s", resp.Status, b)
	}

	return json.NewDecoder(resp.Body).Decode(scanTo)
}

// Render the dashboard.
func dash(url, key string, rng ztime.Range) error {
	type row struct {
		text  string
		color zli.Color
	}
	var (
		left      = make([]row, 0, 16)
		right     = make([]row, 0, 16)
		headerCol = zli.Bold | zli.White | zli.ColorHex("#9a15a4").Bg()
		nr        = func(s string) row { return row{text: s} }
	)

	data, err := getData(url, key, rng)
	if err != nil {
		return err
	}

	// Left column.
	left = append(left, row{
		text:  "Pages" + strings.Repeat(" ", 41),
		color: headerCol,
	})
	for _, path := range data.hits.Hits {
		left = append(left,
			nr(fmt.Sprintf("%-6d  %-38s", path.Count, zli.Colorize(zstring.ElideCenter(path.Path, 37), zli.Bold))),
			nr(fmt.Sprintf("        %-38s", zstring.ElideLeft(path.Title, 37))),
			nr(""),
		)
	}

	// Get browser, system stats.
	renderStat := func(page string) {
		app := make([]row, 0, 4)
		app = append(app, row{
			text:  zstring.UpperFirst(page),
			color: headerCol,
		})
		stats := data.stats[page]

		if len(stats.Stats) == 0 {
			app = append(app, nr("(no data)"))
		}
		for _, stat := range stats.Stats {
			if stat.Name == "" {
				stat.Name = "(unknown)"
			}
			var (
				p    = float64(stat.Count) / float64(data.total.TotalUTC) * 100
				perc string
			)
			switch {
			case p == 0:
				perc = "0%"
			case p < .5:
				perc = fmt.Sprintf("%.1f%%", p)[1:]
			default:
				perc = fmt.Sprintf("%2.0f%%", math.Round(p))
			}
			app = append(app, nr(fmt.Sprintf("%s %s",
				zstring.AlignLeft(zstring.ElideLeft(stat.Name, 23), 24), perc)))
		}

		if page == "toprefs" {
			left = append(left, app...)
		} else {
			if page != "campaigns" { // Last one, don't include blank line.
				app = append(app, nr(""))
			}
			right = append(right, app...)
		}
	}
	for _, page := range []string{"toprefs", "browsers", "systems", "sizes", "locations", "languages", "campaigns"} {
		renderStat(page)
	}

	// Combine the two columns
	fmt.Printf("%s\n", zli.Colorize(zstring.AlignCenter(fmt.Sprintf(
		"%s – %d of %d visits shown", rng.String(), data.hits.Total, data.total.Total),
		78), zli.Bold))
	fmt.Printf("┌%s┬%s┐\n", strings.Repeat("─", 48), strings.Repeat("─", 30))
	m := max(len(left), len(right))
	for i := 0; i < m; i++ {
		l, r := row{}, row{}
		if i < len(right) {
			r = right[i]
		}
		if i < len(left) {
			l = left[i]
		}

		if l.color != 0 {
			fmt.Printf("│%s│", zli.Colorize(" "+termtext.AlignLeft(l.text, 46)+" ", l.color))
		} else {
			fmt.Printf("│ %s │", termtext.AlignLeft(l.text, 46))
		}
		if r.color != 0 {
			fmt.Printf("%s│\n", zli.Colorize(" "+termtext.AlignLeft(r.text, 28)+" ", r.color))
		} else {
			fmt.Printf(" %s │\n", termtext.AlignLeft(r.text, 28))
		}
	}
	fmt.Printf("└%s┴%s┘\n", strings.Repeat("─", 48), strings.Repeat("─", 30))

	return nil
}

type (
	dashboardData struct {
		total goatcounter.TotalCount
		hits  struct {
			Hits  goatcounter.HitLists `json:"hits"`
			Total int                  `json:"total"`
			More  bool                 `json:"more"`
		}
		stats map[string]stats
	}
	stats struct {
		Stats []goatcounter.HitStat `json:"stats"`
		More  bool                  `json:"more"`
	}
)

// Get the required data for the dashboard.
func getData(url, key string, rng ztime.Range) (dashboardData, error) {
	data := dashboardData{stats: make(map[string]stats)}

	// Get totals
	err := doRequest(&data.total, key, "%s/api/v0/stats/total?start=%s&end=%s", url,
		rng.Start.Format("2006-01-02T15:04:05Z"), rng.End.Format("2006-01-02T15:04:05Z"))
	if err != nil {
		return data, err
	}

	// Get pages overview.
	err = doRequest(&data.hits, key, "%s/api/v0/stats/hits?limit=8&start=%s&end=%s", url,
		rng.Start.Format("2006-01-02T15:04:05Z"), rng.End.Format("2006-01-02T15:04:05Z"))
	if err != nil {
		return data, err
	}

	// Get browser, system stats.
	getStat := func(page string) error {
		limit := 4
		if page == "toprefs" {
			limit = 8
		}
		var stats stats
		err = doRequest(&stats, key, "%s/api/v0/stats/%s?limit=%d&start=%s&end=%s", url, page, limit,
			rng.Start.Format("2006-01-02T15:04:05Z"), rng.End.Format("2006-01-02T15:04:05Z"))
		if err != nil {
			return err
		}
		data.stats[page] = stats
		return err
	}
	for _, page := range []string{"toprefs", "browsers", "systems", "sizes", "locations", "languages", "campaigns"} {
		if err := getStat(page); err != nil {
			return data, err
		}
	}

	return data, nil
}

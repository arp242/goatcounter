package main

import (
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"math"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"zgo.at/errors"
	"zgo.at/goatcounter/v2"
	"zgo.at/goatcounter/v2/handlers"
	"zgo.at/goatcounter/v2/logscan"
	"zgo.at/json"
	"zgo.at/zli"
	"zgo.at/zlog"
	"zgo.at/zstd/znet"
	"zgo.at/zstd/zstring"
)

const usageImport = `
Import pageviews from an export or logfile.

Overview:

    This requires a running GoatCounter server; it sends requests to the API
    instead of modifying the database directly. You need to set
    GOATCOUNTER_API_KEY to an API key with "Record pageviews" permissions:

        $ export GOATCOUNTER_API_KEY=[..]
        $ goatcounter import -site=https://stats.example.com export.csv.gz

    You can create an API key with "goatcounter db create apikey -count", or
    from the web interface in "[Username in top menu] → API" from the top-right
    menu.

    You must give one filename to import; use - to read from stdin:

        $ goatcounter import -site=.. export.csv.gz

    Or to keep reading from a log file:

        $ goatcounter import -site=.. -follow /var/log/nginx/access.log

    If you're self-hosting GoatCounter it may be useful to (temporarily)
    increase the ratelimit when importing large files:

        $ goatcounter serve -ratelimit api-count:1000/1 ...

Flags:

  -debug       Modules to debug, comma-separated or 'all' for all modules.
               See "goatcounter help debug" for a list of modules.

  -silent      Don't show progress information.

  -site        Site to import to, as an URL (e.g. "https://stats.example.com")

  -follow      Watch a file for new lines and import them. Existing lines are
               not processed.

  -format      Log format; currently accepted values:

                   csv             GoatCounter CSV export (default)
                   combined        NCSA Combined Log
                   combined-vhost  NCSA Combined Log with virtual host
                   common          Common Log Format (CLF)
                   common-vhost    Common Log Format (CLF) with virtual host
                   log:[fmt]       Custom log format; see "goatcounter help
                                   logfile" for details.

  -date, -time, -datetime
               Format of date and time for log imports; set automatically when
               using one of the predefined log formats and only needs to be set
               when using a custom log:[..]".
               This follows Go's time format; see "goatcounter help logfile" for
               an overview on how this works.

  -exclude     Exclude pageviews that match the given patterns; this flag can be
               given more than once. If no -exclude flag is given then "-exclude
               static -exclude redirect" is used. Use -exclude='' to not exclude
               anything.

               The syntax is [field]:[pattern]; the [field] is one of the fields
               listed in "help logile". The pattern can be prefixed with "glob:"
               or "re:" to get globbing or regular expressions:

                   path:.gif                             Anywhere in the path
                   path:glob:/public/**.{gif,jpg,jpeg}   Any image in /public
                   path:re:\.(gif|jpe?g)$                Any image

               If the first character is "!" then the exclude pattern will be
               inverted: everything that does *not* match the pattern is
               excluded.

               Regular expressions are not anchored by default, so "path:re:/x"
               will match both "/x/y" and "/y/x". Use ^ or $ to anchor them.

               Glob patterns need to match the full string; "path:glob:/x" will
               match only "/x". Use "/x/**" to match everything starting with
               "/x".
               Supported glob patterns: *, **, ?, [..], [!..], {..,..}
               See: https://pkg.go.dev/github.com/bmatcuk/doublestar/v3#Match

               There are the following special values:

                   static      Images, videos, audio files, CSS, and JS files
                               based on the filename and content_type.
                   html        content_type text/html (mostly useful as !html).
                   redirect    Exclude redirects (300-303 responses).

               For example, to exclude all static files, all non-GET requests,
               and everything in the /private/ directory:

                   $ goatcounter import -format combined \
                       -exclude static \
                       -exclude '!method:GET' \
                       -exclude 'path:glob:/private/**' \
                       access_log

Environment:

  GOATCOUNTER_API_KEY   API key; requires "Record pageviews" permission.
`

const helpLogfile = `
Format specifiers are given as $name.

List of format specifiers:

    ignore         Ignore zero or more characters.

    time           Time according to the -time value.
    date           Date according to -date value.
    datetime       Date and time according to -datetime value.

    remote_addr    Client remote address; IPv4 or IPv6 address (DNS names are
                   not supported here).
    xff            Client remote address from X-Forwarded-For header field. The
                   remote address will be set to the last non-private IP
                   address.

    method         Request method.
    status         Status code sent to the client.
    http           HTTP request protocol (i.e. HTTP/1.1).
    path           URL path; this may contain the query string.
    query          Query string; only needed if not included in $path.
    referrer       "Referrer" request header.
    user_agent     User-Agent request header.

Some format specifiers that are not (yet) used anywhere:

    host           Server name of the server serving the request.
    content_type   Content-Type header of the response.
    timing_sec     Time to serve the request in seconds, with possible decimal.
    timing_milli   Time to serve the request in milliseconds.
    timing_micro   Time to serve the request in microseconds.
    size           Size of the object returned to the client.

Date and time parsing:

    Parsing the date and time is done with Go's time package; the following
    placeholders are recognized:

        2006           Year
        Jan            Month name
        1, 01          Month number
        2, 02          Day of month
        3, 03, 15      Hour
        4, 04          Minute
        5, 05          Seconds
        .000000000     Nanoseconds
        MST, -0700     Timezone

    You can give the following pre-defined values:

        ansic          Mon Jan _2 15:04:05 2006
        unix           Mon Jan _2 15:04:05 MST 2006
        unixmilli      1746644937044
        unixsec        1746644937
        rfc822         02 Jan 06 15:04 MST
        rfc822z        02 Jan 06 15:04 -0700
        rfc850         Monday, 02-Jan-06 15:04:05 MST
        rfc1123        Mon, 02 Jan 2006 15:04:05 MST
        rfc1123z       Mon, 02 Jan 2006 15:04:05 -0700
        rfc3339        2006-01-02T15:04:05Z07:00
        rfc3339nano    2006-01-02T15:04:05.999999999Z07:00

    The full documentation is available at https://pkg.go.dev/time
`

func cmdImport(f zli.Flags, ready chan<- struct{}, stop chan struct{}) error {
	var (
		debug    = f.String("", "debug").Pointer()
		site     = f.String("", "site").Pointer()
		format   = f.String("csv", "format").Pointer()
		date     = f.String("", "date").Pointer()
		tyme     = f.String("", "time").Pointer()
		datetime = f.String("", "datetime").Pointer()
		silent   = f.Bool(false, "silent").Pointer()
		follow   = f.Bool(false, "follow").Pointer()
		exclude  = f.StringList(nil, "exclude").Pointer()
	)
	err := f.Parse()
	if err != nil {
		return err
	}

	return func(debug, site, format, date, tyme, datetime string, silent, follow bool, exclude []string) error {
		files := f.Args
		if len(files) == 0 {
			return fmt.Errorf("need a filename")
		}
		if len(files) > 1 {
			return fmt.Errorf("can only specify one filename")
		}

		var fp io.ReadCloser
		if files[0] == "-" {
			fp = io.NopCloser(os.Stdin)
		} else {
			file, err := os.Open(files[0])
			if err != nil {
				return err
			}
			defer file.Close()

			fp = file
			if strings.HasSuffix(files[0], ".gz") {
				fp, err = gzip.NewReader(file)
				if err != nil {
					return errors.Errorf("could not read as gzip: %w", err)
				}
			}
			defer fp.Close()
		}

		zlog.Config.SetDebug(debug)

		url := strings.TrimRight(site, "/")
		if !zstring.HasPrefixes(url, "http://", "https://") {
			url = "https://" + url
		}
		key := os.Getenv("GOATCOUNTER_API_KEY")
		if key == "" {
			return errors.New("GOATCOUNTER_API_KEY must be set")
		}

		err = checkSite(url, key, goatcounter.APIPermCount)
		if err != nil {
			return err
		}

		switch format {
		default:
			err = importLog(fp, ready, stop, url, key, files[0], format, date, tyme, datetime, follow, silent, exclude)
		case "csv":
			ready <- struct{}{}
			if follow {
				return fmt.Errorf("cannot use -follow with -format=csv")
			}
			if len(exclude) > 0 {
				return fmt.Errorf("cannot use -exclude with -format=csv")
			}
			err = importCSV(fp, url, key, silent)
		}
		return err
	}(*debug, *site, *format, *date, *tyme, *datetime, *silent, *follow, *exclude)
}

func importCSV(fp io.ReadCloser, url, key string, silent bool) error {
	n := 0
	ctx := goatcounter.WithSite(context.Background(), &goatcounter.Site{})
	hits := make([]handlers.APICountRequestHit, 0, 500)
	_, err := goatcounter.Import(ctx, fp, false, false, func(hit goatcounter.Hit, final bool) {
		if !final {
			hits = append(hits, handlers.APICountRequestHit{
				Path:      hit.Path,
				Title:     hit.Title,
				Event:     hit.Event,
				Ref:       hit.Ref,
				Size:      hit.Size,
				Bot:       hit.Bot,
				UserAgent: hit.UserAgentHeader,
				Location:  hit.Location,
				CreatedAt: hit.CreatedAt,
				Session:   hit.Session.String(),
			})
		}

		if len(hits) >= 500 || final {
			err := importSend(url, key, silent, false, hits)
			if err != nil {
				fmt.Fprintln(zli.Stdout)
				zli.Errorf(err)
			}

			n += len(hits)
			if !silent {
				zli.Replacef("Imported %d rows", n)
			}

			hits = make([]handlers.APICountRequestHit, 0, 500)
		}
	})
	return err
}

func importLog(
	fp io.ReadCloser,
	ready chan<- struct{}, stop <-chan struct{},
	url, key, file, format, date, tyme, datetime string, follow, silent bool, exclude []string,
) error {
	var (
		scan *logscan.Scanner
		err  error
	)
	if follow && file != "-" {
		fp.Close()
		scan, err = logscan.NewFollow(context.Background(), file, format, date, tyme, datetime, exclude)
	} else {
		scan, err = logscan.New(fp, format, date, tyme, datetime, exclude)
	}
	if err != nil {
		return err
	}

	hits := make(chan handlers.APICountRequestHit, 100)

	// Persist every 10 seconds because it may take a while for 100 pageviews to
	// arrive when using -follow.
	d := 10 * time.Second // TODO: add flag for this, so it's easier to test.
	t := time.NewTicker(d)

	go func() {
		for {
			<-t.C
			persistLog(hits, url, key, silent, follow)
		}
	}()

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		<-stop
		cancel()
	}()

	defer persistLog(hits, url, key, silent, follow)
	ready <- struct{}{}
	n := 0
	for {
		line, raw, lineno, err := scan.Line(ctx)
		if err == io.EOF {
			break
		}
		if err != nil {
			fmt.Fprintln(zli.Stdout)
			return err
		}

		hit := handlers.APICountRequestHit{
			Line:      raw,
			LineNo:    lineno,
			Path:      line.Path(),
			Ref:       line.Referrer(),
			Query:     line.Query(),
			UserAgent: line.UserAgent(),
		}

		hit.CreatedAt, err = line.Datetime(scan)
		if err != nil {
			zlog.Error(err)
			continue
		}

		if line.XForwardedFor() != "" {
			xffSplit := strings.Split(line.XForwardedFor(), ",")
			for i := len(xffSplit) - 1; i >= 0; i-- {
				if !znet.PrivateIP(net.ParseIP(xffSplit[i])) {
					hit.IP = znet.RemovePort(strings.TrimSpace(xffSplit[i]))
					break
				}
			}
		}
		if hit.IP == "" {
			hit.IP = znet.RemovePort(line.RemoteAddr())
		}

		hits <- hit
		if len(hits) >= cap(hits) {
			n += len(hits)
			t.Reset(d)
			persistLog(hits, url, key, silent, follow)
			if !silent && !follow {
				zli.Replacef("Imported %d rows", n)
			}
		}
	}
	return nil
}

// Send everything off if we have 100 entries or if 10 seconds expired,
// whichever happens first.
func persistLog(hits <-chan handlers.APICountRequestHit, url, key string, silent, follow bool) {
	l := len(hits)
	if l == 0 {
		return
	}
	collect := make([]handlers.APICountRequestHit, l)
	for i := 0; i < l; i++ {
		collect[i] = <-hits
	}

	err := importSend(url, key, silent, follow, collect)
	if err != nil {
		zlog.Error(err)
	}
}

var importClient = http.Client{Timeout: 5 * time.Second}

func importSend(url, key string, silent, follow bool, hits []handlers.APICountRequestHit) error {
	body, err := json.Marshal(handlers.APICountRequest{NoSessions: true, Hits: hits})
	if err != nil {
		return err
	}

	i := 0
retry:
	r, err := newRequest("POST", url+"/api/v0/count", key, bytes.NewReader(body))
	if err != nil {
		return err
	}
	r.Header.Set("X-Goatcounter-Import", "yes")

	zlog.Module("import-api").Debugf("POST %s with %d hits", url, len(hits))
	resp, err := importClient.Do(r)
	if err != nil {
		if i > 5 {
			return err
		}

		i++
		w := (time.Duration(math.Pow(float64(i), 2)) * time.Second).Round(time.Second)
		fmt.Fprintf(zli.Stderr, "non-fatal error; retrying in %s: %s\n", w, err)
		time.Sleep(w)
		goto retry
	}
	defer resp.Body.Close()

	showError := func(fatal bool) error {
		var b []byte
		b, _ = io.ReadAll(resp.Body)
		var gcErr struct {
			Errors map[int]string `json:"errors"`
		}
		jsErr := json.Unmarshal(b, &gcErr)
		if jsErr == nil {
			for i, e := range gcErr.Errors {
				zlog.Fields(zlog.F{
					"lineno": hits[i].LineNo,
					"line":   strings.TrimRight(hits[i].Line, "\r\n"),
					"error":  strings.TrimSpace(e),
				}).Errorf("error processing line %d", hits[i].LineNo)
			}
		}
		err := fmt.Errorf("%s: %s: %s", url, resp.Status, b)
		if fatal {
			return err
		}
		fmt.Fprintln(zli.Stderr, err.Error())
		return nil
	}

	switch resp.StatusCode {
	case 200, 202:
		// Success, do nothing.
	case http.StatusTooManyRequests:
		s, _ := strconv.Atoi(resp.Header.Get("X-Rate-Limit-Reset"))
		if !silent {
			fmt.Fprintf(zli.Stdout, "\nwaiting %d seconds for the ratelimiter\n", s)
		}
		time.Sleep(time.Duration(s) * time.Second)
		return importSend(url, key, silent, follow, hits)
	case 400:
		return showError(false)
	default:
		return showError(true)
	}
	return nil
}

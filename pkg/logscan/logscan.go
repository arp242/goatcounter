package logscan

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"math"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/bmatcuk/doublestar/v4"
	"zgo.at/errors"
	"zgo.at/follow"
	"zgo.at/goatcounter/v2/pkg/log"
)

var reFormat = regexp.MustCompile(`\\\$[\w-_]+`)

const (
	fieldAcceptLanguage = "accept_language"
	fieldContentType    = "content_type"
	fieldHost           = "host"
	fieldHTTP           = "http"
	fieldMethod         = "method"
	fieldPath           = "path"
	fieldQuery          = "query"
	fieldReferrer       = "referrer"
	fieldRemoteAddr     = "remote_addr"
	fieldSize           = "size"
	fieldStatus         = "status"
	fieldUserAgent      = "user_agent"
	fieldXff            = "xff"
)

var fields = []string{"ignore", "time", "date", "datetime", fieldRemoteAddr,
	fieldXff, fieldMethod, fieldStatus, fieldHTTP, fieldPath, fieldQuery, fieldReferrer,
	fieldUserAgent, fieldHost, fieldContentType, "timing_sec", "timing_milli",
	"timing_micro", fieldSize}

const (
	excludeContains = 0
	excludeGlob     = 1
	excludeRe       = 2
)

type LineParser interface {
	Parse(string) (Line, bool, error)
}

type Line interface {
	Host() string
	RemoteAddr() string
	XForwardedFor() string
	Method() string
	HTTP() string
	Path() string
	Query() string
	Referrer() string
	UserAgent() string
	ContentType() string
	Status() int
	Size() int
	Language() string

	Timing() (time.Duration, error)
	Datetime(lp LineParser) (time.Time, error)
}

const (
	// Combined format; used by default in Apache, nginx.
	//
	// 127.0.0.1    user -  [10/Oct/2000:13:55:36 -0700] "GET     /path HTTP/1.1" 200     2326  "https://ref" "Mozilla/5.0"
	// $remote_addr $ignore [$datetime]                  "$method $path $http"    $status $size "$referrer"   "$user_agent"
	Combined      = `$remote_addr $ignore [$datetime] "$method $path $http" $status $size "$referrer" "$user_agent"`
	CombinedVhost = `$host:` + Combined

	// Common log format.
	//
	// 127.0.0.1    user -  [10/Oct/2000:13:55:36 -0700] "GET     /path HTTP/1.1" 200     2326
	// $remote_addr $ignore [$datetime]                  "$method $path $http"    $status $size`
	Common      = `$remote_addr $ignore [$datetime] "$method $path $http" $status $size`
	CommonVhost = `$host:` + Common

	// Bunny; works for both the regular and "extended" format.
	// https://docs.bunny.net/docs/cdn-log-format
	Bunny = `$ignore|$status|$datetime|$size|$host|$remote_addr|$referrer|$url|$ignore|$user_agent|$ignore|$ignore`
)

func getFormat(format, date, time, datetime string) (string, string, string, string) {
	if strings.HasPrefix(format, "log:") {
		return format[4:], date, time, datetime
	}

	switch strings.ToLower(format) {
	case "combined":
		return Combined, "", "", "02/Jan/2006:15:04:05 -0700"
	case "combined-vhost":
		return CombinedVhost, "", "", "02/Jan/2006:15:04:05 -0700"
	case "common":
		return Common, "", "", "02/Jan/2006:15:04:05 -0700"
	case "common-vhost":
		return CommonVhost, "", "", "02/Jan/2006:15:04:05 -0700"
	case "bunny", "bunny-extended":
		return Bunny, "", "", "unix_milli"
	default:
		return "", "", "", ""
	}
}

type Scanner struct {
	read   chan follow.Data
	lineno uint64
	lp     LineParser
}

// New processes all the lines in the reader.
func New(in io.Reader, format, date, tyme, datetime string, exclude []string) (*Scanner, error) {
	s, err := makeNew(format, date, tyme, datetime, exclude)
	if err != nil {
		return nil, fmt.Errorf("logscan.New: %w", err)
	}

	data := make(chan follow.Data)
	go func() {
		scan := bufio.NewScanner(in)
		for scan.Scan() {
			data <- follow.Data{Bytes: append([]byte(nil), scan.Bytes()...)}
		}
		data <- follow.Data{Err: io.EOF}
	}()
	s.read = data
	return s, nil
}

// NewFollow follows a file for new lines and processes them. Existing lines are
// not processed.
func NewFollow(ctx context.Context, file, format, date, tyme, datetime string, exclude []string) (*Scanner, error) {
	s, err := makeNew(format, date, tyme, datetime, exclude)
	if err != nil {
		return nil, fmt.Errorf("logscan.NewFollow: %w", err)
	}

	f := follow.New()
	go func() {
		err := f.Start(ctx, file)
		if err != nil {
			log.Error(ctx, errors.Errorf("logscan.NewFollow: %w", err))
		}
	}()
	s.read = f.Data
	return s, nil
}

func makeNew(format, date, tyme, datetime string, exclude []string) (*Scanner, error) {
	excludePatt, err := processExcludes(exclude)
	if err != nil {
		return nil, err
	}

	var p LineParser
	if format == "caddy" {
		p, err = newCaddyParser(datetime, excludePatt)
	} else {
		p, err = newRegexParser(format, date, tyme, datetime, excludePatt)
	}
	if err != nil {
		return nil, err
	}
	return &Scanner{lp: p}, nil
}

// Line processes a single line.
func (s *Scanner) Line(ctx context.Context) (Line, string, uint64, error) {
start:
	var line string
	select {
	case <-ctx.Done():
		return nil, "", 0, io.EOF
	case r := <-s.read:
		if r.Err != nil {
			return nil, "", 0, r.Err
		}
		line = r.String()
		s.lineno++
	}

	parsed, excluded, err := s.lp.Parse(line)
	if err != nil {
		return nil, "", 0, err
	}
	// Could use "return Line(ctx) as well, but if many lines are excluded
	// that will run out of stack space. So just restart the function from
	// the top waiting for the s.read channel.
	if excluded {
		goto start
	}

	return parsed, line, s.lineno, nil
}

func (s *Scanner) Datetime(l Line) (time.Time, error) {
	return l.Datetime(s.lp)
}

type excludePattern struct {
	kind    int            // exclude* constant
	negate  bool           // ! present
	field   string         // "path", "content_type"
	pattern string         // ".gif", "*.gif"
	re      *regexp.Regexp // only if kind=excludeRe
}

func processExcludes(exclude []string) ([]excludePattern, error) {
	// "static" needs to expand to two values.
	for i, e := range exclude {
		switch e {
		case "static":
			// Note: maybe check if using glob patterns is faster?
			exclude[i] = `path:re:.*\.(:?js|css|gif|jpe?g|png|svg|ico|web[mp]|mp[34])$`
			exclude = append(exclude, `content_type:re:^(?:text/(?:css|javascript)|image/(?:png|gif|jpeg|svg\+xml|webp)).*?`)
		case "html":
			exclude[i] = "content_type:^text/html.*?"
		case "redirect":
			exclude[i] = "status:glob:30[0123]"
		}
	}

	patterns := make([]excludePattern, 0, len(exclude))
	for _, e := range exclude {
		var p excludePattern
		if strings.HasPrefix(e, "!") {
			p.negate = true
			e = e[1:]
		}

		p.field, p.pattern, _ = strings.Cut(e, ":")
		if !slices.Contains(fields, p.field) {
			return nil, fmt.Errorf("invalid field %q in exclude pattern %q", p.field, e)
		}
		if p.pattern == "" {
			return nil, fmt.Errorf("no pattern in %q", e)
		}

		var err error
		switch {
		case strings.HasPrefix(p.pattern, "glob:"):
			p.kind, p.pattern = excludeGlob, p.pattern[5:]
			_, err = doublestar.Match(p.pattern, "")
		case strings.HasPrefix(p.pattern, "re:"):
			p.kind, p.pattern = excludeRe, p.pattern[3:]
			p.re, err = regexp.Compile(p.pattern)
		}
		if err != nil {
			return nil, fmt.Errorf("invalid exclude pattern: %q: %w", e, err)
		}
		patterns = append(patterns, p)
	}

	return patterns, nil
}

func matchesPattern(e excludePattern, v string) bool {
	var m bool
	switch e.kind {
	default:
		m = strings.Contains(v, e.pattern)
	case excludeGlob:
		// We use doublestar instead of filepath.Match() because the latter
		// doesn't support "**" and "{a,b}" patterns, both of which are very
		// useful here.
		m, _ = doublestar.Match(e.pattern, v)
	case excludeRe:
		m = e.re.MatchString(v)
	}
	if e.negate {
		return !m
	}
	return m
}

func parseDatetime(format, s string, f float64) (time.Time, error) {
	var (
		t   time.Time
		n   int64
		err error
	)
	switch format {
	case "unix_sec":
		n, err = strconv.ParseInt(s, 10, 64)
		t = time.Unix(n, 0)
	case "unix_milli":
		n, err = strconv.ParseInt(s, 10, 64)
		t = time.UnixMilli(n)
	case "unix_nano":
		if f == 0 {
			n, err = strconv.ParseInt(s, 10, 64)
		} else {
			n = int64(f)
		}
		s := n / 1e9
		t = time.Unix(s, n-s*1e9)
	case "unix_sec_float":
		if f == 0 {
			f, err = strconv.ParseFloat(s, 64)
		}
		sec, dec := math.Modf(f)
		t = time.Unix(int64(sec), int64(dec*1e9))
	case "unix_milli_float":
		if f == 0 {
			f, err = strconv.ParseFloat(s, 64)
		}
		sec, dec := math.Modf(f / 1000)
		t = time.Unix(int64(sec), int64(dec*1e9))
	default:
		t, err = time.Parse(format, s)
	}
	return t.UTC(), err
}

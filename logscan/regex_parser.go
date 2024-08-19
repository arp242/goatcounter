package logscan

import (
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/bmatcuk/doublestar/v4"
	"zgo.at/errors"
)

type RegexParser struct {
	re      *regexp.Regexp
	names   []string
	exclude []excludePattern

	date, time, datetime string
}

// Returns the structured (Line, shouldExclude, err)
func (p RegexParser) Parse(line string) (Line, bool, error) {
	parsed := make(RegexLine, len(p.names)+2)
	for _, sub := range p.re.FindAllStringSubmatchIndex(line, -1) {
		for i := 2; i < len(sub); i += 2 {
			v := line[sub[i]:sub[i+1]]
			if v == "-" { // Using - is common to indicate a blank value.
				v = ""
			}
			parsed[p.names[i/2]] = v
		}
	}
	for _, e := range p.exclude {
		if parsed.exclude(e) {
			return nil, true, nil
		}
	}

	return parsed, false, nil
}

var _ LineParser = RegexParser{}

func newRegexParser(format, date, tyme, datetime string, exclude []string) (*RegexParser, error) {
	of := format
	format, date, tyme, datetime = getFormat(format, date, tyme, datetime)
	if format == "" {
		return nil, errors.Errorf("unknown format: %s", of)
	}

	excludePatt, err := processExcludes(exclude)
	if err != nil {
		return nil, err
	}

	pat := reFormat.ReplaceAllStringFunc(regexp.QuoteMeta(format), func(m string) string {
		m = m[2:]

		p := ".+?"
		switch m {
		default:
			err = fmt.Errorf("unknown format specifier: $%s", m)
		case "ignore":
			return ".*?"

		case "date":
			if date == "" {
				err = errors.New("$date used but -date value is empty")
			} else {
				_, err = time.Parse(date, date)
				if err != nil {
					err = errors.Errorf("invalid -date format: %s", err)
				}
			}
		case "time":
			if tyme == "" {
				err = errors.New("$time used but -time value is empty")
			} else {
				_, err = time.Parse(tyme, tyme)
				if err != nil {
					err = errors.Errorf("invalid -time format: %s", err)
				}
			}
		case "datetime":
			if datetime == "" {
				err = errors.New("$datetime used but -datetime value is empty")
			} else {
				_, err = time.Parse(datetime, datetime)
				if err != nil {
					err = errors.Errorf("invalid -datetime format: %s", err)
				}
			}

		case "host":
			p = `(?:xn--)?[a-zA-Z0-9.-]+`
		case "remote_addr":
			p = `[0-9a-fA-F:.]+`
		case "xff":
			p = `[0-9a-fA-F:. ,]+`

		case "method":
			p = `[A-Z]{3,10}`
		case "status":
			p = `\d{3}`
		case "http":
			p = `HTTP/[\d.]+`
		case "path":
			p = `/.*?`
		case "timing_sec":
			p = `[\d.]+`
		case "timing_milli", "timing_micro":
			p = `\d+`
		case "size":
			p = `(?:\d+|-)`
		case "referrer", "user_agent":
			p = `.*?`
		case "query", "content_type":
			// Default
		}
		return "(?P<" + m + ">" + p + ")"
	})
	if err != nil {
		return nil, fmt.Errorf("invalid -format value: %w", err)
	}
	re, err := regexp.Compile("^" + pat + "$")
	return &RegexParser{
		re:       re,
		names:    re.SubexpNames(),
		date:     date,
		time:     tyme,
		datetime: datetime,
		exclude:  excludePatt,
	}, nil
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

type RegexLine map[string]string

func (l RegexLine) Host() string          { return l["host"] }
func (l RegexLine) RemoteAddr() string    { return l["remote_addr"] }
func (l RegexLine) XForwardedFor() string { return l["xff"] }
func (l RegexLine) Method() string        { return l["method"] }
func (l RegexLine) HTTP() string          { return l["http"] }
func (l RegexLine) Path() string          { return l["path"] }
func (l RegexLine) Query() string         { return l["query"] }
func (l RegexLine) Referrer() string      { return l["referrer"] }
func (l RegexLine) UserAgent() string     { return l["user_agent"] }
func (l RegexLine) ContentType() string   { return l["content_type"] }
func (l RegexLine) Status() int           { return toI(l["status"]) }
func (l RegexLine) Size() int             { return toI(l["size"]) }
func (l RegexLine) Language() string      { return l["accept_language"] }

func (l RegexLine) Timing() time.Duration {
	s, ok := l["timing_sec"]
	if ok {
		return time.Duration(toI(s)) * time.Second
	}
	s, ok = l["timing_milli"]
	if ok {
		return time.Duration(toI64(s)) * time.Millisecond
	}
	s, ok = l["timing_micro"]
	if ok {
		return time.Duration(toI64(s)) * time.Microsecond
	}
	return 0
}

func (l RegexLine) Datetime(scan *Scanner) (time.Time, error) {
	parser := scan.lp.(*RegexParser)
	s, ok := l["date"]
	if ok {
		t, err := time.Parse(parser.date, s)
		return t.UTC(), err
	}
	s, ok = l["time"]
	if ok {
		t, err := time.Parse(parser.time, s)
		return t.UTC(), err
	}
	s, ok = l["datetime"]
	if ok {
		t, err := time.Parse(parser.datetime, s)
		return t.UTC(), err
	}
	return time.Time{}, nil
}

func toI(s string) int {
	n, _ := strconv.Atoi(s) // Regexp only captures \d, so safe to ignore.
	return n
}
func toI64(s string) int64 {
	n, _ := strconv.ParseInt(s, 10, 64)
	return n
}

var _ Line = RegexLine{}

func (l RegexLine) exclude(e excludePattern) bool {
	var m bool
	switch e.kind {
	default:
		m = strings.Contains(l[e.field], e.pattern)
	case excludeGlob:
		// We use doublestar instead of filepath.Match() because the latter
		// doesn't support "**" and "{a,b}" patterns, both of which are very
		// useful here.
		m, _ = doublestar.Match(e.pattern, l[e.field])
	case excludeRe:
		m = e.re.MatchString(l[e.field])
	}
	if e.negate {
		return !m
	}
	return m
}

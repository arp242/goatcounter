package logscan

import (
	"fmt"
	"regexp"
	"time"

	"zgo.at/errors"
)

type LineParser interface {
	Parse(string) (Line, bool, error)
}

type RegexParser struct {
	re      *regexp.Regexp
	names   []string
	exclude []excludePattern

	date, time, datetime string
}

// Returns the structured (Line, shouldExclude, err)
func (p RegexParser) Parse(line string) (Line, bool, error) {
	parsed := make(Line, len(p.names)+2)
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

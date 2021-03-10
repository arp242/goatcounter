// Copyright © 2019 Martin Tournoij <martin@arp242.net>
// This file is part of GoatCounter and published under the terms of the EUPL
// v1.2, which can be found in the LICENSE file or at http://eupl12.zgo.at

package logscan

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
	"time"

	"zgo.at/errors"
	"zgo.at/follow"
	"zgo.at/zlog"
)

var reFormat = regexp.MustCompile(`\\\$[\w-_]+`)

func processFormat(format, date, tyme, datetime string) (*regexp.Regexp, string, string, string, error) {
	of := format
	format, date, tyme, datetime = getFormat(format, date, tyme, datetime)
	if format == "" {
		return nil, "", "", "", errors.Errorf("unknown format: %s", of)
	}

	var err error
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
			p = `/.*?` // `/.*[^ ]`
		case "query":
		case "referrer":
		case "user_agent":

		case "timing_sec":
			p = `[\d.]+`
		case "timing_milli", "timing_micro":
			p = `\d+`
		case "size":
			p = `\d+`
		}
		return "(?P<" + m + ">" + p + ")"
	})
	if err != nil {
		return nil, "", "", "", fmt.Errorf("invalid -format value: %w", err)
	}
	re, err := regexp.Compile(pat)
	return re, date, tyme, datetime, err
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
	default:
		return "", "", "", ""
	}
}

type Scanner struct {
	read  chan follow.Data
	re    *regexp.Regexp
	names []string

	date, time, datetime string
}

// New processes all the lines in the reader.
func New(in io.Reader, format, date, tyme, datetime string) (*Scanner, error) {
	re, date, tyme, datetime, err := processFormat(format, date, tyme, datetime)
	if err != nil {
		return nil, errors.Errorf("logscan.New: %w", err)
	}

	data := make(chan follow.Data)
	scan := bufio.NewScanner(in)
	go func() {
		for scan.Scan() {
			data <- follow.Data{Bytes: scan.Bytes()}
		}
		data <- follow.Data{Err: io.EOF}
	}()

	return &Scanner{read: data, re: re, names: re.SubexpNames(), date: date, time: tyme, datetime: datetime}, nil
}

// NewFollow follows a file for new lines and processes them. Existing lines are
// not processed.
func NewFollow(ctx context.Context, file string, format, date, tyme, datetime string) (*Scanner, error) {
	re, date, tyme, datetime, err := processFormat(format, date, tyme, datetime)
	if err != nil {
		return nil, errors.Errorf("logscan.New: %w", err)
	}

	f := follow.New()
	go func() {
		err := f.Start(ctx, file)
		if err != nil {
			zlog.Error(errors.Errorf("logscan.NewFollow: %w", err))
		}
	}()

	return &Scanner{read: f.Data, re: re, names: re.SubexpNames(), date: date, time: tyme, datetime: datetime}, nil
}

func (s Scanner) DateFormats() (date, time, datetime string) {
	return s.date, s.time, s.datetime
}

// Line processes a single line.
func (s Scanner) Line(ctx context.Context) (Line, error) {
	var line string
	select {
	case <-ctx.Done():
		return Line{}, io.EOF
	case r := <-s.read:
		if r.Err != nil {
			return nil, r.Err
		}
		line = r.String()
	}

	parsed := make(Line, len(s.names))
	for _, sub := range s.re.FindAllStringSubmatchIndex(line, -1) {
		for i := 2; i < len(sub); i += 2 {
			v := line[sub[i]:sub[i+1]]
			if v == "-" { // Using - is common to indicate a blank value.
				v = ""
			}
			parsed[s.names[i/2]] = v
		}
	}
	return parsed, nil
}

func toI(s string) int {
	n, _ := strconv.Atoi(s) // Regexp only captures \d, so safe to ignore.
	return n
}
func toI64(s string) int64 {
	n, _ := strconv.ParseInt(s, 10, 64)
	return n
}

type Line map[string]string

func (l Line) Host() string          { return l["host"] }
func (l Line) RemoteAddr() string    { return l["remote_addr"] }
func (l Line) XForwardedFor() string { return l["xff"] }
func (l Line) Method() string        { return l["method"] }
func (l Line) HTTP() string          { return l["http"] }
func (l Line) Path() string          { return l["path"] }
func (l Line) Query() string         { return l["query"] }
func (l Line) Referrer() string      { return l["referrer"] }
func (l Line) UserAgent() string     { return l["user_agent"] }
func (l Line) Status() int           { return toI(l["status"]) }
func (l Line) Size() int             { return toI(l["size"]) }

func (l Line) Timing() time.Duration {
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

func (l Line) Datetime(scan *Scanner) (time.Time, error) {
	date, tyme, datetime := scan.DateFormats()

	s, ok := l["date"]
	if ok {
		t, err := time.Parse(date, s)
		return t.UTC(), err
	}
	s, ok = l["time"]
	if ok {
		t, err := time.Parse(tyme, s)
		return t.UTC(), err
	}
	s, ok = l["datetime"]
	if ok {
		t, err := time.Parse(datetime, s)
		return t.UTC(), err
	}
	return time.Time{}, nil
}

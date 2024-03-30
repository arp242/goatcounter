// Copyright © Martin Tournoij – This file is part of GoatCounter and published
// under the terms of a slightly modified EUPL v1.2 license, which can be found
// in the LICENSE file or at https://license.goatcounter.com

package logscan

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"regexp"
	"strings"
	"time"

	"zgo.at/errors"
	"zgo.at/follow"
	"zgo.at/zlog"
)

var reFormat = regexp.MustCompile(`\\\$[\w-_]+`)

var fields = []string{"ignore", "time", "date", "datetime", "remote_addr",
	"xff", "method", "status", "http", "path", "query", "referrer",
	"user_agent", "host", "content_type", "timing_sec", "timing_milli",
	"timing_micro", "size"}

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

	Timing() time.Duration
	Datetime(scan *Scanner) (time.Time, error)
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
			zlog.Error(errors.Errorf("logscan.NewFollow: %w", err))
		}
	}()
	s.read = f.Data
	return s, nil
}

func makeNew(format, date, tyme, datetime string, exclude []string) (*Scanner, error) {
	p, err := newRegexParser(format, date, tyme, datetime, exclude)
	if err != nil {
		return nil, err
	}

	return &Scanner{
		lp: p,
	}, nil
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

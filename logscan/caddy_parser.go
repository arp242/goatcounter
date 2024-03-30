package logscan

import (
	"encoding/json"
	"fmt"
	"net/url"
	"time"
)

// https://caddyserver.com/docs/caddyfile/directives/log
type (
	CaddyParser struct {
		datetime        string
		excludePatterns []excludePattern
	}
	CaddyLogEntry struct {
		Timestamp   any          `json:"ts"`
		Request     CaddyRequest `json:"request"`
		Duration    any          `json:"duration"`
		Size_       int          `json:"size"`
		Status_     int          `json:"status"`
		RespHeaders CaddyHeaders `json:"resp_headers"`
	}
	CaddyRequest struct {
		RemoteAddr string       `json:"remote_addr"`
		Proto      string       `json:"proto"`
		Method     string       `json:"method"`
		Host       string       `json:"host"`
		URI        string       `json:"uri"`
		Headers    CaddyHeaders `json:"headers"`
	}
	CaddyHeaders struct {
		UserAgent      []string `json:"User-Agent"`
		Referer        []string `json:"Referer"`
		ContentType    []string `json:"Content-Type"`
		XForwardedFor  []string `json:"X-Forwarded-For"`
		AcceptLanguage []string `json:"Accept-Language"`
	}
)

func newCaddyParser(datetime string, exclude []excludePattern) (CaddyParser, error) {
	if datetime == "" {
		datetime = "unix_sec_float" // Caddy default
	}
	return CaddyParser{datetime: datetime, excludePatterns: exclude}, nil
}

func (p CaddyParser) Parse(line string) (Line, bool, error) {
	var logEntry CaddyLogEntry
	err := json.Unmarshal([]byte(line), &logEntry)
	if err != nil {
		return nil, false, err
	}

	for _, e := range p.excludePatterns {
		if logEntry.matchesPattern(e) {
			return nil, true, nil
		}
	}
	return logEntry, false, nil
}

func (l CaddyLogEntry) Host() string       { return l.Request.Host }
func (l CaddyLogEntry) RemoteAddr() string { return l.Request.RemoteAddr }
func (l CaddyLogEntry) Method() string     { return l.Request.Method }
func (l CaddyLogEntry) HTTP() string       { return l.Request.Proto }
func (l CaddyLogEntry) Status() int        { return l.Status_ }
func (l CaddyLogEntry) Size() int          { return l.Size_ }
func (l CaddyLogEntry) Path() string {
	u, err := url.ParseRequestURI(l.Request.URI)
	if err != nil {
		return ""
	}
	return u.Path
}

func (l CaddyLogEntry) Query() string {
	u, err := url.Parse(l.Request.URI)
	if err != nil {
		return ""
	}
	return u.RawQuery
}

// Only supports the "seconds" and "string" values of duration_format. "millis"
// or "nanos" will give the wrong result. It's okay for now, as this is never
// actually used (yet).
func (l CaddyLogEntry) Timing() (time.Duration, error) {
	switch d := l.Duration.(type) {
	case float64:
		return time.Duration(d * float64(time.Second)), nil
	case string:
		return time.ParseDuration(d)
	default:
		return 0, fmt.Errorf("unknown duration type %T for %#[1]v", l.Duration)
	}
}

func (l CaddyLogEntry) Datetime(lp LineParser) (time.Time, error) {
	parser := lp.(CaddyParser)
	switch parser.datetime {
	case "unix_nano", "unix_sec_float", "unix_milli_float":
		return parseDatetime(parser.datetime, "", l.Timestamp.(float64))
	}
	return parseDatetime(parser.datetime, l.Timestamp.(string), 0)
}
func (l CaddyLogEntry) XForwardedFor() string {
	if len(l.Request.Headers.XForwardedFor) > 0 {
		return l.Request.Headers.XForwardedFor[0]
	}
	return ""
}
func (l CaddyLogEntry) Referrer() string {
	if len(l.Request.Headers.Referer) > 0 {
		return l.Request.Headers.Referer[0]
	}
	return ""
}
func (l CaddyLogEntry) UserAgent() string {
	if len(l.Request.Headers.UserAgent) > 0 {
		return l.Request.Headers.UserAgent[0]
	}
	return ""
}
func (l CaddyLogEntry) ContentType() string {
	if len(l.Request.Headers.ContentType) > 0 {
		return l.Request.Headers.ContentType[0]
	}
	return ""
}
func (l CaddyLogEntry) Language() string {
	if len(l.Request.Headers.AcceptLanguage) > 0 {
		return l.Request.Headers.AcceptLanguage[0]
	}
	return ""
}

func (l CaddyLogEntry) fieldValue(name string) string {
	switch name {
	default:
		panic(fmt.Sprintf("Received invalid field request: %s", name))
	case fieldUserAgent:
		return l.UserAgent()
	case fieldHost:
		return l.Host()
	case fieldRemoteAddr:
		return l.RemoteAddr()
	case fieldAcceptLanguage:
		return l.Language()
	case fieldContentType:
		return l.ContentType()
	case fieldHTTP:
		return l.HTTP()
	case fieldMethod:
		return l.Method()
	case fieldPath:
		return l.Path()
	case fieldQuery:
		return l.Query()
	case fieldReferrer:
		return l.Referrer()
	case fieldSize:
		return fmt.Sprint(l.Size())
	case fieldStatus:
		return fmt.Sprint(l.Status())
	case fieldXff:
		return l.XForwardedFor()
	}
}

func (l CaddyLogEntry) matchesPattern(e excludePattern) bool {
	return matchesPattern(e, l.fieldValue(e.field))
}

package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"zgo.at/goatcounter/v2"
	"zgo.at/zstd/zmap"
	"zgo.at/zstd/ztest"
)

func fmtCSP(h string) string {
	csp := make(map[string][]string)
	for _, f := range strings.Split(h, ";") {
		s := strings.Fields(f)
		if len(s) > 1 {
			csp[s[0]] = s[1:]
		}
	}
	keys, l := zmap.LongestKey(csp)
	slices.Sort(keys)
	var s strings.Builder
	for _, k := range keys {
		s.WriteString(k)
		s.WriteString(strings.Repeat(" ", l-len(k)+1))
		s.WriteString(strings.Join(csp[k], " "))
		s.WriteByte('\n')
	}
	return s.String()
}

func TestAddCSP(t *testing.T) {
	tests := []struct {
		path  string
		embed []string
		want  string
	}{
		{"/", nil, `
			connect-src     'self' wss:
			default-src     'none'
			font-src        'self' https://gc.zgo.at
			form-action     'self'
			frame-ancestors 'none'
			frame-src       'self'
			img-src         'self' https://gc.zgo.at data:
			manifest-src    'self' https://gc.zgo.at
			script-src      'self' https://gc.zgo.at
			style-src       'self' https://gc.zgo.at 'unsafe-inline'
		`},
		{"/api.html", nil, `
			connect-src     'self' wss:
			default-src     'none'
			font-src        'self' https://gc.zgo.at 'unsafe-inline'
			form-action     'self'
			frame-ancestors 'none'
			frame-src       'self'
			img-src         'self' https://gc.zgo.at 'unsafe-inline' data:
			manifest-src    'self' https://gc.zgo.at 'unsafe-inline'
			script-src      'self' https://gc.zgo.at 'unsafe-inline'
			style-src       'self' https://gc.zgo.at 'unsafe-inline' 'unsafe-inline'
		`},
		{"/", []string{"http://example.com example.net"}, `
			connect-src     'self' wss:
			default-src     'none'
			font-src        'self' https://gc.zgo.at
			form-action     'self'
			frame-ancestors http://example.com example.net
			frame-src       'self'
			img-src         'self' https://gc.zgo.at data:
			manifest-src    'self' https://gc.zgo.at
			script-src      'self' https://gc.zgo.at
			style-src       'self' https://gc.zgo.at 'unsafe-inline'
		`},
		{"/", []string{"http://example.com"}, `
			connect-src     'self' wss:
			default-src     'none'
			font-src        'self' https://gc.zgo.at
			form-action     'self'
			frame-ancestors http://example.com
			frame-src       'self'
			img-src         'self' https://gc.zgo.at data:
			manifest-src    'self' https://gc.zgo.at
			script-src      'self' https://gc.zgo.at
			style-src       'self' https://gc.zgo.at 'unsafe-inline'
		`},
		{"/count", []string{"http://example.com"}, ``},
	}

	mw := addcsp("")(http.NewServeMux())
	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			var (
				ctx = goatcounter.WithSite(context.Background(), &goatcounter.Site{Settings: goatcounter.SiteSettings{AllowEmbed: tt.embed}})
				r   = ztest.NewRequest("GET", tt.path, nil).WithContext(ctx)
				rr  = httptest.NewRecorder()
			)

			mw.ServeHTTP(rr, r)

			tt.want = ztest.NormalizeIndent(tt.want)
			have := fmtCSP(rr.Header().Get("Content-Security-Policy"))
			if d := ztest.Diff(have, tt.want); d != "" {
				t.Error(d)
			}
		})
	}
}

func BenchmarkAddCSP(b *testing.B) {
	var (
		ctx = goatcounter.WithSite(context.Background(), &goatcounter.Site{})
		r   = ztest.NewRequest("GET", "/", nil).WithContext(ctx)
		rr  = httptest.NewRecorder()
		mw  = addcsp("")(http.NewServeMux())
	)
	b.ResetTimer()
	for b.Loop() {
		mw.ServeHTTP(rr, r)
	}
}

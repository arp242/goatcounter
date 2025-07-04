package acme_test

import (
	"testing"

	. "zgo.at/goatcounter/v2/acme"
	"zgo.at/goatcounter/v2/gctest"
	"zgo.at/zdb"
	"zgo.at/zhttp"
	"zgo.at/zstd/zruntime"
)

func TestSetup(t *testing.T) {
	tests := []struct {
		flag string

		wantTLS  bool
		wantACME bool
		wantFlag uint8
	}{
		// No TLS.
		{"", false, false, 0},
		{"http", false, false, 0},

		//flagTLS = map[bool]string{true: "none", false: "acme"}[dev]

		{"acme,http", false, true, 0}, // saas default
		{"acme,proxy", false, true, 0},
		{"acme,rdr", true, true, zhttp.ServeRedirect}, // serve default

		{"acme:some/dir,rdr", true, true, zhttp.ServeRedirect},

		{"acme,testdata/test.pem", true, true, 0},
		{"testdata/test.pem", true, false, 0},
		{"rdr,testdata/test.pem", true, false, zhttp.ServeRedirect},
	}

	for _, tt := range tests {
		t.Run(tt.flag, func(t *testing.T) {
			defer Reset()
			ctx := gctest.DB(t)

			tlsC, acmeH, haveFlag := Setup(zdb.MustGetDB(ctx), tt.flag, true)
			haveTLS := tlsC != nil
			haveACME := acmeH != nil

			if tlsC != nil {
				t.Log(zruntime.FuncName(tlsC.GetCertificate))
			}
			if haveTLS != tt.wantTLS {
				t.Errorf("have TLS %t; want %t", haveTLS, tt.wantTLS)
			}
			if haveACME != tt.wantACME {
				t.Errorf("have ACME %t; want %t", haveACME, tt.wantACME)
			}
			if haveFlag != tt.wantFlag {
				t.Errorf("have flag %d; want %d", haveFlag, tt.wantFlag)
			}
		})
	}
}

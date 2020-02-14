package handlers

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net/http"
	"strings"

	"golang.org/x/crypto/acme/autocert"
	"zgo.at/goatcounter"
	"zgo.at/utils/sliceutil"
	"zgo.at/zdb"
	"zgo.at/zhttp"
)

func SetupTLS(db zdb.DB, flag string) (*tls.Config, http.HandlerFunc, uint8) {
	if flag == "" {
		return nil, nil, 0
	}

	s := strings.Split(flag, ",")
	if len(s) == 0 || sliceutil.InStringSlice(s, "none") {
		return nil, nil, 0
	}
	if len(s) == 0 {
		panic(fmt.Sprintf("wrong value for -tls: %q", flag))
	}

	var (
		listen uint8
		certs  []tls.Certificate
		m      *autocert.Manager
	)
	for _, f := range s {
		switch f {
		case "":
			panic(fmt.Sprintf("wrong value for -tls: %q", flag))
		case "tls":
			listen += zhttp.ServeTLS
		case "rdr":
			listen += zhttp.ServeRedirect
		case "acme":
			m = &autocert.Manager{
				Cache:  autocert.DirCache("secret-dir"),
				Prompt: autocert.AcceptTOS,
				HostPolicy: func(ctx context.Context, host string) error {
					var s goatcounter.Sites
					ok, err := s.HasCNAME(zdb.With(ctx, db), host)
					if err != nil {
						return err
					}
					if !ok {
						return fmt.Errorf("HasCNAME: unknown host: %q", host)
					}
					return nil
				},
			}
		default:
			if !strings.HasSuffix(f, ".pem") {
				panic(fmt.Sprintf("wrong value for -tls: %q", f))
			}
			cert, err := tls.LoadX509KeyPair(f, f)
			if err != nil {
				panic(err)
			}
			leaf, err := x509.ParseCertificate(cert.Certificate[0])
			if err != nil {
				panic(err)
			}
			cert.Leaf = leaf
			certs = append(certs, cert)
		}
	}

	if m == nil {
		if len(certs) == 0 {
			panic("-tls: no acme and no certificates")
		}
		return &tls.Config{Certificates: certs}, nil, listen
	}

	tlsc := m.TLSConfig()
	if len(certs) > 0 {
		tlsc.GetCertificate = func(hello *tls.ClientHelloInfo) (*tls.Certificate, error) {
			for _, c := range certs {
				if c.Leaf.VerifyHostname(hello.ServerName) == nil {
					return &c, nil
				}
			}

			return m.GetCertificate(hello)
		}
	}

	return tlsc, m.HTTPHandler(nil).ServeHTTP, listen
}

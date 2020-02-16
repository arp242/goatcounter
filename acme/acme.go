package acme

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net/http"
	"strings"

	"github.com/pkg/errors"
	"golang.org/x/crypto/acme/autocert"
	"zgo.at/goatcounter"
	"zgo.at/utils/sliceutil"
	"zgo.at/zdb"
	"zgo.at/zhttp"
)

var manager *autocert.Manager

// cache is like autocert.DirCache, but ensures that files end with .pem
type cache struct{ dc autocert.DirCache }

func NewCache(dir string) cache                                     { return cache{dc: autocert.DirCache(dir)} }
func (d cache) Get(ctx context.Context, key string) ([]byte, error) { return d.dc.Get(ctx, key) }
func (d cache) Delete(ctx context.Context, key string) error        { return d.dc.Delete(ctx, key) }
func (d cache) Put(ctx context.Context, name string, data []byte) error {
	name += ".pem"
	return d.dc.Put(ctx, name, data)
}

// Setup returns a tls.Config and http-01 verification based on the value of the
// -tls cmdline flag.
func Setup(db zdb.DB, flag string) (*tls.Config, http.HandlerFunc, uint8) {
	if flag == "" {
		return nil, nil, 0
	}

	s := strings.Split(flag, ",")
	if len(s) == 0 || sliceutil.InStringSlice(s, "none") {
		return nil, nil, 0
	}

	var (
		listen uint8
		certs  []tls.Certificate
	)
	for _, f := range s {
		switch {
		default:
			panic(fmt.Sprintf("wrong value for -tls: %q", f))
		case f == "":
			panic(fmt.Sprintf("wrong value for -tls: %q", flag))
		case f == "tls":
			listen += zhttp.ServeTLS
		case f == "rdr":
			listen += zhttp.ServeRedirect
		case strings.HasSuffix(f, ".pem"):
			cert, err := tls.LoadX509KeyPair(f, f)
			if err != nil {
				panic(err)
			}
			if len(cert.Certificate) == 0 {
				panic(fmt.Sprintf("no certificates in %q", f))
			}
			if len(cert.Certificate) > 1 {
				panic(fmt.Sprintf("multiple certificates in %q", f))
			}

			leaf, err := x509.ParseCertificate(cert.Certificate[0])
			if err != nil {
				panic(err)
			}
			cert.Leaf = leaf
			certs = append(certs, cert)
		case strings.HasPrefix(f, "acme"):
			dir := "acme-secrets"
			if c := strings.Index(f, ":"); c > -1 {
				dir = f[c+1:]
			}

			manager = &autocert.Manager{
				Cache:  NewCache(dir),
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
		}
	}

	if manager == nil {
		if len(certs) == 0 {
			panic("-tls: no acme and no certificates")
		}
		return &tls.Config{Certificates: certs}, nil, listen
	}

	tlsc := manager.TLSConfig()
	if len(certs) > 0 {
		// The standard GetCertificate() prefers ACME over the .pem files, but
		// this isn't what we want for goatcounter.com since we have a
		// DNS-verified *.goatcounter.com ACME certificate that we want to load
		// first, and then fall back to the "custom.domain.com" ACME certs.
		//
		// The .pem files are managed externally, because dns-01 verification is
		// a bit tricky and not really something that needs to be part of
		// GoatCounter.
		tlsc.GetCertificate = func(hello *tls.ClientHelloInfo) (*tls.Certificate, error) {
			for _, c := range certs {
				if c.Leaf.VerifyHostname(hello.ServerName) == nil {
					return &c, nil
				}
			}
			return manager.GetCertificate(hello)
		}
	}

	return tlsc, manager.HTTPHandler(nil).ServeHTTP, listen
}

// Make a new certificate for the domain.
func Make(domain string) error {
	if manager == nil {
		panic("acme.MakeCert: no manager, use Setup() first")
	}

	hello := &tls.ClientHelloInfo{
		ServerName:        domain,
		SupportedProtos:   []string{"h2", "http/1.1"},
		SupportedVersions: []uint16{tls.VersionTLS13, tls.VersionTLS12, tls.VersionTLS11},
		CipherSuites: []uint16{
			tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
			tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,
		},
	}

	_, err := manager.GetCertificate(hello)
	return errors.Wrapf(err, "acme.MakeCert for %q", domain)
}

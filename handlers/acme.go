package handlers

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"strings"

	"golang.org/x/crypto/acme/autocert"
	"zgo.at/goatcounter"
	"zgo.at/zdb"
	"zgo.at/zhttp"
)

func SetupTLS(db zdb.DB, flag string) (*tls.Config, http.HandlerFunc, uint8) {
	if flag == "" {
		return nil, nil, 0
	}

	s := strings.Split(flag, ",")
	if len(s) == 0 || len(s) > 3 {
		panic(fmt.Sprintf("wrong value for -tls: %q", flag))
	}

	listen := uint8(0)
	for _, f := range s[1:] {
		switch f {
		case "tls":
			listen += zhttp.ServeTLS
		case "rdr":
			listen += zhttp.ServeRedirect
		default:
			panic(fmt.Sprintf("wrong value for -tls: %q", flag))
		}
	}

	switch s[0] {
	case "":
		panic(fmt.Sprintf("wrong value for -tls: %q", flag))

	case "acme":
		m := &autocert.Manager{
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

		return m.TLSConfig(), m.HTTPHandler(nil).ServeHTTP, listen

	default:
		cert, err := tls.LoadX509KeyPair(s[0], s[0])
		if err != nil {
			panic(err)
		}
		return &tls.Config{Certificates: []tls.Certificate{cert}}, nil, listen
	}
}

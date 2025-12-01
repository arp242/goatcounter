package geo

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	_ "embed"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"zgo.at/goatcounter/v2/pkg/geo/geoip2"
	"zgo.at/goatcounter/v2/pkg/log"
)

//go:embed GeoLite2-Country.mmdb.gz
var bundle []byte

var ctxkey = &struct{ n string }{"geo"}

func With(ctx context.Context, db *geoip2.Reader) context.Context {
	return context.WithValue(ctx, ctxkey, db)
}

func Get(ctx context.Context) *geoip2.Reader {
	db, ok := ctx.Value(ctxkey).(*geoip2.Reader)
	if !ok {
		return nil
	}
	return db
}

// Open a geoDB database located at the given path.
//
// The database can be the "Countries" or "Cities" version.
//
// It will use the embeded "Countries" database if path is an empty string.
//
// It will download a database if the path starts with "maxmind:". This needs to
// be as "maxmind:accountID:licenseKey[:path]", where :path is optional and
// detaults to goatcounter-data/auto.mmdb.
func Open(path string) (*geoip2.Reader, error) {
	// Use built-in
	if path == "" {
		gz, err := gzip.NewReader(bytes.NewReader(bundle))
		if err != nil {
			return nil, err
		}
		d, err := io.ReadAll(gz)
		if err != nil {
			return nil, err
		}
		db, err := geoip2.FromBytes(d)
		if err != nil {
			return nil, err
		}
		return db, nil
	}

	bundle = nil // Not using it; save some memory.

	// Download update
	if strings.HasPrefix(path, "maxmind:") {
		s := strings.Split(path[8:], ":")
		if l := len(s); l != 2 && l != 3 {
			return nil, fmt.Errorf("invalid format for MaxMind GeoIP update: %q", path)
		}
		accountID, key, dst := s[0], s[1], "goatcounter-data/auto.mmdb"
		if len(s) == 3 {
			dst = s[2]
		}
		st, err := os.Stat(dst)
		if err != nil || st.ModTime().Before(time.Now().Add(-24*time.Hour*7)) {
			log.Module("startup").Info(context.Background(), "downloading GeoDB database; might take a few seconds")
			err = fetchDB(accountID, key, dst)
			if err != nil {
				return nil, err
			}
		}
		path = dst
	}

	// From FS
	return geoip2.Open(path)
}

func fetch(accountID, key, p string) ([]byte, error) {
	var (
		c    = http.Client{Timeout: 10 * time.Second}
		r, _ = http.NewRequest("GET", p, nil)
	)
	r.SetBasicAuth(accountID, key)
	r.Header.Add("User-Agent", "GoatCounter/1.0 (+https://github.com/arp242/goatcounter)")
	resp, err := c.Do(r)
	if err != nil {
		return nil, fmt.Errorf("fetching %q: %w", p, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(resp.Body)
		if len(b) > 500 {
			b = append(b[:500], []byte("…")...)
		}
		return nil, fmt.Errorf("fetching %q: %s: %s", p, resp.Status, string(b))
	}
	return io.ReadAll(resp.Body)
}

func fetchHash(accountID, key string) (string, error) {
	p := "https://download.maxmind.com/geoip/databases/GeoLite2-City/download?suffix=tar.gz.sha256"
	b, err := fetch(accountID, key, p)
	if err != nil {
		return "", err
	}

	f := strings.Fields(string(b))
	if len(f) != 2 {
		return "", fmt.Errorf("unexpected return for %q: %s", p, string(b))
	}
	return f[0], nil
}

func fetchDB(accountID, key, path string) error {
	hash, err := fetchHash(accountID, key)
	if err != nil {
		return err
	}

	err = os.MkdirAll(filepath.Dir(path), 0o777)
	if err != nil {
		return err
	}

	tmp, err := os.CreateTemp(filepath.Dir(path), "update-geodb-*")
	if err != nil {
		return err
	}
	defer func() {
		tmp.Close()
		os.Remove(tmp.Name())
	}()

	p := "https://download.maxmind.com/geoip/databases/GeoLite2-City/download?suffix=tar.gz"
	b, err := fetch(accountID, key, p)
	if err != nil {
		return err
	}

	h := sha256.New()
	h.Write(b)
	if hh := fmt.Sprintf("%x", h.Sum(nil)); hh != hash {
		return fmt.Errorf("hash mismatch for %q: have %s, want %s", p, hh, hash)
	}
	gz, err := gzip.NewReader(bytes.NewReader(b))
	if err != nil {
		return fmt.Errorf("reading %q: %w", p, err)
	}
	defer gz.Close()
	archive := tar.NewReader(gz)

	for {
		h, err := archive.Next()
		if err != nil {
			// Don't ignore io.EOF, as reaching this means we haven't seen a
			// mmdb file
			return fmt.Errorf("reading %q: %w", p, err)
		}
		if strings.HasSuffix(h.Name, ".mmdb") {
			_, err := io.Copy(tmp, archive)
			if err != nil {
				return fmt.Errorf("writing %q: %w", tmp.Name(), err)
			}
			if err := tmp.Close(); err != nil {
				return fmt.Errorf("writing %q: %w", tmp.Name(), err)
			}
			return os.Rename(tmp.Name(), path)
		}
	}
}

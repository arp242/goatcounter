package handlers

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io/fs"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
	"zgo.at/goatcounter/v2"
	"zgo.at/guru"
	"zgo.at/zcache/v2"
	"zgo.at/zdb"
	"zgo.at/zhttp"
	"zgo.at/zstd/zfilepath"
	"zgo.at/zstd/zfs"
	"zgo.at/zstd/ztime"
	"zgo.at/ztpl/tplfunc"
)

type vcounter struct{ files fs.FS }

func (h vcounter) mount(r chi.Router) {
	// This relies on e.g. Varnish for more extended caching.
	c := r.With(middleware.Compress(2), func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Cache-Control", "public")
			w.Header().Set("Expires", ztime.Now(r.Context()).Add(4*time.Hour).Format(time.RFC1123Z))
			next.ServeHTTP(w, r)
		})
	})

	c.Get("/counter/*", zhttp.Wrap(h.counter))
}

var (
	loadVCFilesOnce                          sync.Once
	html, htmlNoBranding, svg, svgNoBranding string
	pngImg                                   image.Image
	pngImgTotal                              image.Image
	fontFace                                 font.Face
	fontColor                                *image.Uniform
)

func loadVCFiles(fsys fs.FS) {
	html = string(zfs.MustReadFile(fsys, "vcounter/vcounter.html"))
	svg = string(zfs.MustReadFile(fsys, "vcounter/vcounter.svg"))

	htmlNoBranding = strings.NewReplacer(
		`<br><span id="gcvc-by">stats by GoatCounter</span>`, "",
		"76px", "56px").Replace(html)
	svgNoBranding = strings.NewReplacer(
		`<text id="gcvc-by" x="50%%" y="70">stats by GoatCounter</text>`, "",
		`height="80" viewBox="0 0 200 80"`, `height="60" viewBox="0 0 200 60"`,
		`height="78"`, `height="58"`).Replace(svg)

	// Image
	f, err := opentype.Parse(gobold.TTF)
	if err != nil {
		panic(err)
	}
	fontFace, err = opentype.NewFace(f, &opentype.FaceOptions{
		Size:    22,
		DPI:     72,
		Hinting: font.HintingFull,
	})
	if err != nil {
		panic(err)
	}
	fontColor = image.NewUniform(color.RGBA{R: 0x9a, G: 0x15, B: 0xa4, A: 255})

	pngImg, _, err = image.Decode(bytes.NewReader(zfs.MustReadFile(fsys, "vcounter/vcounter.png")))
	if err != nil {
		panic(err)
	}

	pngImgTotal, _, err = image.Decode(bytes.NewReader(zfs.MustReadFile(fsys, "vcounter/vcounter-total.png")))
	if err != nil {
		panic(err)
	}
}

type vcache struct {
	out      []byte
	ct       string
	notfound bool

	// TODO: GetStale() gets stale entries (we want this), and GetWithExpired()
	// also gets the expiration date (we want this too), but there is no way to
	// have both.
	//
	// I think we want GetWithExpired() to just return stale content, too?
	// Anyway, for now waste a bit of memory.
	created time.Time
}

var (
	vcounterCache  = zcache.New[string, vcache](time.Hour*4, time.Hour*8)
	vcounterUpdate = zcache.New[string, struct{}](zcache.NoExpiration, zcache.NoExpiration)
)

func (h vcounter) get(ctx context.Context, path, ext string, q url.Values, total bool) (bool, []byte, string, error) {
	var (
		site       = Site(ctx)
		noBranding = q.Get("no_branding") != ""
		style      = q.Get("style")
	)

	var (
		rng      ztime.Range
		err      error
		startArg = q.Get("start")
	)
	if startArg != "" {
		switch startArg {
		case "week":
			rng.Start = ztime.Now(ctx).Add(-7 * 24 * time.Hour)
		case "month":
			rng.Start = ztime.Now(ctx).Add(-30 * 24 * time.Hour)
		case "year":
			rng.Start = ztime.Now(ctx).Add(-365 * 24 * time.Hour)
		default:
			rng.Start, err = time.Parse("2006-01-02", startArg)
		}
		if err != nil {
			return false, nil, "", guru.WithCode(400, err)
		}
	}
	if s := q.Get("end"); s != "" {
		rng.End, err = time.Parse("2006-01-02", s)
		if err != nil {
			return false, nil, "", guru.WithCode(400, err)
		}
	}

	var hl goatcounter.HitList
	if total {
		err = hl.SiteTotalUTC(ctx, rng)
	} else {
		err = hl.PathCount(ctx, path, rng)
	}
	if err != nil && !zdb.ErrNoRows(err) {
		return false, nil, "", err
	}
	var notfound bool
	if zdb.ErrNoRows(err) {
		notfound = true
	}

	count := tplfunc.Number(hl.Count, site.UserDefaults.NumberFormat)
	switch ext {
	default:
		return false, nil, "", guru.Errorf(400, "unknown extension: %q", ext)
	case "json":
		return notfound, []byte(`{"count_unique":"` + count + `", "count":"` + count + `"}`), "application/json", nil
	case "html":
		s := html
		if noBranding {
			s = htmlNoBranding
		}
		if total {
			s = strings.Replace(s, "page", "site", 1)
		}
		return notfound, []byte(fmt.Sprintf(s, style, count)), "text/html;charset=utf-8", nil
	case "svg":
		s := svg
		if noBranding {
			s = svgNoBranding
		}
		if total {
			s = strings.Replace(s, "page", "site", 1)
		}
		return notfound, []byte(fmt.Sprintf(s, style, count)), "image/svg+xml", nil
	case "png":
		src := pngImg
		if total {
			src = pngImgTotal
		}

		bounds := src.Bounds()
		if noBranding {
			bounds.Max.Y = 60
		}
		img := image.NewRGBA(bounds)
		for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
			for x := bounds.Min.X; x < bounds.Max.X; x++ {
				img.Set(x, y, src.At(x, y))
			}
		}
		if noBranding { // Copy bottom border.
			for y := 58; y <= 59; y++ {
				for x := bounds.Min.X; x < bounds.Max.X; x++ {
					img.Set(x, y, src.At(x, y+20))
				}
			}
		}

		// Draw to temporary image first, so we know the size of the result.
		// Then copy that in the destination.
		tmp := image.NewRGBA(bounds)
		drw := font.Drawer{
			Dst:  tmp,
			Src:  fontColor,
			Face: fontFace,
			Dot:  fixed.P(0, 22),
		}

		// TODO: the PNG doesn't seem to draw the thin space, so just use a
		// regular space.
		count = strings.ReplaceAll(count, "\u202f", " ")
		drw.DrawString(count)

		start := 100 - drw.Dot.X.Round()/2
		bounds = tmp.Bounds()
		for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
			for x := bounds.Min.X; x < bounds.Max.X; x++ {
				c := tmp.At(x, y)
				if _, _, _, a := c.RGBA(); a != 0 {
					img.Set(start+x, y+24, c)
				}
			}
		}

		buf := new(bytes.Buffer)
		err = png.Encode(buf, img)
		if err != nil {
			return false, nil, "", err
		}
		return notfound, buf.Bytes(), "image/png", nil
	}
}

func (h vcounter) counter(w http.ResponseWriter, r *http.Request) error {
	loadVCFilesOnce.Do(func() { loadVCFiles(h.files) })

	site := Site(r.Context())
	if !site.Settings.AllowCounter {
		return guru.New(http.StatusForbidden, "Need to enable the ‘allow using the visitor counter’ setting")
	}

	var (
		q         = r.URL.Query()
		path, ext = zfilepath.SplitExt(r.URL.Path[9:])
		total     = path == "TOTAL"
	)
	// Sanitize slashes, but only if we can't find the path (so events work).
	if !total {
		var p goatcounter.Path
		err := p.ByPath(r.Context(), path)
		if err != nil {
			if !zdb.ErrNoRows(err) {
				return err
			}
			path = "/" + strings.Trim(path, "/")
		}
	}

	// Don't need to take options in to account for cache. Some people are using
	// "cache buster" URL params so can't use all of r.URL.
	cachekey := strconv.FormatInt(int64(site.ID), 10) + "-" + path + "." + ext + "-" + q.Get("start") + "-" + q.Get("end")

	getcount := func() (vcache, error) {
		notfound, out, ct, err := h.get(r.Context(), path, ext, q, total)
		if err != nil {
			return vcache{}, err
		}
		c := vcache{out: out, ct: ct, created: time.Now(), notfound: notfound}

		vcounterCache.Set(cachekey, c)
		return c, nil

	}
	send := func(c vcache) error {
		w.Header().Set("Content-Type", c.ct)
		if c.ct == "application/json" {
			w.Header().Set("Access-Control-Allow-Origin", "*")
		}
		w.Header().Set("Age", strconv.Itoa(int(time.Since(c.created).Seconds())))
		if c.notfound {
			w.WriteHeader(404)
		}
		w.Write(c.out)
		return nil
	}

	cached, expired, ok := vcounterCache.GetStale(cachekey)
	if ok {
		// Cache expired: still use the old cache, but start a goroutine to update it.
		if expired {
			if _, ok := vcounterUpdate.Get(cachekey); !ok {
				vcounterUpdate.Set(cachekey, struct{}{})
				go func() {
					defer vcounterUpdate.Delete(cachekey)
					getcount()
				}()
			}
		}
		return send(cached)
	}

	c, err := getcount()
	if err != nil {
		return err
	}
	return send(c)
}

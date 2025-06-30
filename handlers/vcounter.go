package handlers

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io/fs"
	"net/http"
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
			w.Header().Set("Expires", ztime.Now(r.Context()).Add(30*time.Minute).Format(time.RFC1123Z))
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

func (h vcounter) counter(w http.ResponseWriter, r *http.Request) error {
	loadVCFilesOnce.Do(func() { loadVCFiles(h.files) })

	site := Site(r.Context())
	if !site.Settings.AllowCounter {
		return guru.New(http.StatusForbidden, "Need to enable the ‘allow using the visitor counter’ setting")
	}

	var (
		path, ext  = zfilepath.SplitExt(r.URL.Path[9:])
		total      = path == "TOTAL"
		noBranding = r.URL.Query().Get("no_branding") != ""
		style      = r.URL.Query().Get("style")
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

	if ext == "json" {
		w.Header().Set("Access-Control-Allow-Origin", "*")
	}

	var (
		rng      ztime.Range
		err      error
		startArg = r.URL.Query().Get("start")
	)
	if startArg != "" {
		switch startArg {
		case "week":
			rng.Start = ztime.Now(r.Context()).Add(-7 * 24 * time.Hour)
		case "month":
			rng.Start = ztime.Now(r.Context()).Add(-30 * 24 * time.Hour)
		case "year":
			rng.Start = ztime.Now(r.Context()).Add(-365 * 24 * time.Hour)
		default:
			rng.Start, err = time.Parse("2006-01-02", startArg)
		}
		if err != nil {
			return guru.WithCode(400, err)
		}
	}
	if s := r.URL.Query().Get("end"); s != "" {
		rng.End, err = time.Parse("2006-01-02", s)
		if err != nil {
			return guru.WithCode(400, err)
		}
	}

	var hl goatcounter.HitList
	if total {
		err = hl.SiteTotalUTC(r.Context(), rng)
	} else {
		err = hl.PathCount(r.Context(), path, rng)
	}
	if err != nil && !zdb.ErrNoRows(err) {
		return err
	}
	if zdb.ErrNoRows(err) {
		w.WriteHeader(404)
	}
	count := tplfunc.Number(hl.Count, site.UserDefaults.NumberFormat)

	switch ext {
	default:
		return guru.Errorf(400, "unknown extension: %q", ext)
	case "json":
		return zhttp.JSON(w, map[string]string{
			"count_unique": count,
			"count":        count,
		})
	case "html":
		w.Header().Set("Content-Type", "text/html; charset=utf-8")

		s := html
		if noBranding {
			s = htmlNoBranding
		}
		if total {
			s = strings.Replace(s, "page", "site", 1)
		}

		return zhttp.String(w, fmt.Sprintf(s, style, count))
	case "svg":
		w.Header().Set("Content-Type", "image/svg+xml")

		s := svg
		if noBranding {
			s = svgNoBranding
		}
		if total {
			s = strings.Replace(s, "page", "site", 1)
		}

		return zhttp.String(w, fmt.Sprintf(s, style, count))
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

		w.Header().Set("Content-Type", "image/png")
		return png.Encode(w, img)
	}
}

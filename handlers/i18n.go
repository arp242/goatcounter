package handlers

import (
	"cmp"
	"context"
	"fmt"
	"io/fs"
	"net/http"
	"slices"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/go-chi/chi/v5"
	"golang.org/x/text/language"
	"zgo.at/blackmail"
	"zgo.at/goatcounter/v2"
	"zgo.at/goatcounter/v2/pkg/log"
	"zgo.at/guru"
	"zgo.at/z18n/msgfile"
	"zgo.at/zdb"
	"zgo.at/zhttp"
)

type i18n struct {
	saveHandler func() // TODO: use
	formatLink  func(string) string
}

func Newi18n() i18n {
	return i18n{
		saveHandler: nil,
		formatLink:  func(l string) string { return strings.Replace(l, ":", "#L", 1) }, // GitHub links.
	}
}

func (h i18n) mount(r chi.Router) {
	r.Get("/i18n", zhttp.Wrap(h.list))
	r.Get("/i18n/{file}", zhttp.Wrap(h.show))
	r.Post("/i18n/{file}", zhttp.Wrap(h.save))
	r.Post("/i18n", zhttp.Wrap(h.new))
	r.Post("/i18n/submit/{file}", zhttp.Wrap(h.submit))
}

func (h i18n) list(w http.ResponseWriter, r *http.Request) error {
	fsys := goatcounter.TranslationFiles

	ls, err := fs.ReadDir(fsys, ".")
	if err != nil {
		return err
	}

	count := func(file string) (int, error) { // TODO: should be in in z18n
		fp, err := fsys.Open(file)
		if err != nil {
			return 0, err
		}
		defer fp.Close()
		var f msgfile.File
		_, err = toml.NewDecoder(fp).Decode(&f)
		if err != nil {
			return 0, err
		}
		if file == "template.toml" { // Should always have a default, but just in case.
			return len(f.Strings), nil
		}
		var n int
		for _, s := range f.Strings {
			if s.Default != "" {
				n++
			}
		}
		return n, nil
	}

	totalstr, err := count("template.toml")
	if err != nil {
		return err
	}

	type x struct {
		Name string
		Num  int
		Mod  bool
	}
	files := make([]x, 0, len(ls))
	for _, f := range ls {
		// TODO: don't hard-code; should add msgfile.ReadDir()
		if f.Name() == "template.toml" || f.Name() == "en-GB.toml" {
			continue
		}

		n, err := count(f.Name())
		if err != nil {
			return err
		}

		files = append(files, x{Name: f.Name(), Num: n})
	}

	var fromdb goatcounter.Translations
	err = fromdb.List(r.Context())
	if err != nil {
		return err
	}
outer:
	for _, e := range fromdb {
		var n int
		for _, s := range e.Strings {
			if s.Default != "" {
				n++
			}
		}

		for i, f := range files {
			if f.Name != e.Language+".toml" {
				continue
			}

			f.Mod = true
			f.Num = n
			files[i] = f
			continue outer
		}

		files = append(files, x{
			Name: e.Language + ".toml",
			Num:  n,
			Mod:  true,
		})
	}
	slices.SortFunc(files, func(a, b x) int { return cmp.Compare(a.Name, b.Name) })

	return zhttp.Template(w, "i18n_list.gohtml", struct {
		Globals
		TotalStrings int
		Files        []x
	}{newGlobals(w, r), totalstr, files})
}

func (h i18n) show(w http.ResponseWriter, r *http.Request) error {
	file := chi.URLParam(r, "file")

	var t goatcounter.Translation
	err := t.ByFilename(r.Context(), file)
	if err != nil && !zdb.ErrNoRows(err) {
		return err
	}
	if zdb.ErrNoRows(err) {
		file, err := msgfile.ReadFile(goatcounter.TranslationFiles, file)
		if err != nil {
			return err
		}
		t = goatcounter.Translation(file)
	}

	// TODO: don't hard-code, and add msgfile.Dir type so we can operate on that.
	base, err := msgfile.ReadFile(goatcounter.TranslationFiles, "template.toml")
	if err != nil {
		return err
	}

	tml, err := msgfile.File(t).TOML()
	if err != nil {
		return err
	}

	return zhttp.Template(w, "i18n_show.gohtml", struct {
		Globals
		BaseFile   msgfile.File
		File       goatcounter.Translation
		ShowTOML   bool
		TOML       string
		Filename   string
		FormatLink func(string) string
	}{newGlobals(w, r), base, t, r.URL.Query().Has("toml"), tml, file, h.formatLink})
}

func (h i18n) new(w http.ResponseWriter, r *http.Request) error {
	var args struct {
		Language string `json:"language"`
	}
	if _, err := zhttp.Decode(r, &args); err != nil {
		return err
	}

	// Go's language tag parser accepts both _ and -, but JavaScript doesn't.
	args.Language = strings.ReplaceAll(args.Language, "_", "-")
	if _, err := language.Parse(args.Language); err != nil {
		return guru.Errorf(400, "%q is not a valid language tag", args.Language)
	}

	file, err := msgfile.New(goatcounter.TranslationFiles, args.Language)
	if err != nil {
		return err
	}

	t := goatcounter.Translation(file)
	err = t.Store(r.Context(), args.Language+".toml")
	if err != nil {
		return err
	}

	zhttp.Flash(w, r, fmt.Sprintf("%q added", args.Language))
	return zhttp.SeeOther(w, "/i18n")
}

func (h i18n) save(w http.ResponseWriter, r *http.Request) error {
	file := chi.URLParam(r, "file")
	var args struct {
		Language string        `json:"language"`
		Entry    msgfile.Entry `json:"entry"`
	}
	_, err := zhttp.Decode(r, &args)
	if err != nil {
		return err
	}

	r.Header.Set("Content-Type", "application/json") // Hack to return JSON from ErrPage.

	var t goatcounter.Translation
	err = t.ByFilename(r.Context(), file)
	if err != nil && !zdb.ErrNoRows(err) {
		return err
	}
	if zdb.ErrNoRows(err) {
		orig, err := msgfile.ReadFile(goatcounter.TranslationFiles, file)
		if err != nil {
			return err
		}
		t = goatcounter.Translation(orig)
	}

	s := t.Strings[args.Entry.ID]
	s.Default = args.Entry.Default
	s.Zero = args.Entry.Zero
	s.One = args.Entry.One
	s.Two = args.Entry.Two
	s.Few = args.Entry.Few
	s.Many = args.Entry.Many
	t.Strings[args.Entry.ID] = s

	err = t.Store(r.Context(), file)
	if err != nil {
		return err
	}

	return zhttp.JSON(w, map[string]any{"success": true})
}

func (h i18n) submit(w http.ResponseWriter, r *http.Request) error {
	file := chi.URLParam(r, "file")

	var t goatcounter.Translation
	err := t.ByFilename(r.Context(), file)
	if err != nil {
		return err
	}
	tt, err := msgfile.File(t).TOML()
	if err != nil {
		return err
	}
	msg := fmt.Sprintf("User: %d; language: %q\n\n%s", User(r.Context()).ID, file, tt)

	ctx := context.WithoutCancel(r.Context())
	go func() {
		err := blackmail.Get(ctx).Send("GoatCounter translation updates",
			blackmail.From("", User(ctx).Email),
			blackmail.To("support@goatcounter.com"),
			blackmail.BodyText([]byte(msg)))
		if err != nil {
			log.Error(ctx, err)
		}
	}()

	zhttp.Flash(w, r, "email sent to support@goatcounter.com; I'll take a look as soon as possible.")
	return zhttp.SeeOther(w, "/i18n/"+file)
}

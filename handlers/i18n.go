// Copyright © Martin Tournoij – This file is part of GoatCounter and published
// under the terms of a slightly modified EUPL v1.2 license, which can be found
// in the LICENSE file or at https://license.goatcounter.com

package handlers

import (
	"fmt"
	"io/fs"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"golang.org/x/text/language"
	"zgo.at/blackmail"
	"zgo.at/goatcounter/v2"
	"zgo.at/guru"
	"zgo.at/z18n/msgfile"
	"zgo.at/zhttp"
	"zgo.at/zlog"
	"zgo.at/zstd/zfilepath"
	"zgo.at/zstd/zfs"
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
	r.Post("/i18n/set/{file}", zhttp.Wrap(h.set))
	r.Post("/i18n/submit/{file}", zhttp.Wrap(h.submit))
}

func (h i18n) list(w http.ResponseWriter, r *http.Request) error {
	fsys := goatcounter.Translations(r.Context())

	ls, err := fs.ReadDir(fsys, ".")
	if err != nil {
		return err
	}

	files := make([][2]string, 0, len(ls))
	for _, f := range ls {
		// TODO: don't hard-code; should add msgfile.ReadDir()
		if f.Name() == "template.toml" || f.Name() == "en-GB.toml" {
			continue
		}

		name := f.Name()
		if o, ok := fsys.(zfs.OverlayFS); ok && o.InOverlay(f.Name()) {
			if o.InBase(f.Name()) {
				name += " (modified)"
			} else {
				name += " (new)"
			}
		}

		files = append(files, [2]string{f.Name(), name})
	}

	return zhttp.Template(w, "i18n_list.gohtml", struct {
		Globals
		Files [][2]string
	}{newGlobals(w, r), files})
}

func (h i18n) show(w http.ResponseWriter, r *http.Request) error {
	fsys := goatcounter.Translations(r.Context())

	file, err := msgfile.ReadFile(fsys, chi.URLParam(r, "file"))
	if err != nil {
		return err
	}

	// TODO: don't hard-code, and add msgfile.Dir type so we can operate on that.
	base, err := msgfile.ReadFile(fsys, "template.toml")
	if err != nil {
		return err
	}

	return zhttp.Template(w, "i18n_show.gohtml", struct {
		Globals
		Base       msgfile.File
		File       msgfile.File
		TOMLFile   string
		FormatLink func(string) string
	}{newGlobals(w, r), base, file, chi.URLParam(r, "file"), h.formatLink})
}

func (h i18n) new(w http.ResponseWriter, r *http.Request) error {
	var args struct {
		Language string `json:"language"`
	}
	_, err := zhttp.Decode(r, &args)
	if err != nil {
		return err
	}

	// Go's language tag parser accepts both _ and -, but JavaScript doesn't.
	args.Language = strings.ReplaceAll(args.Language, "_", "-")

	if _, err := language.Parse(args.Language); err != nil {
		return guru.Errorf(400, "%q is not a valid language tag", args.Language)
	}

	file, err := msgfile.New(goatcounter.Translations(r.Context()), args.Language)
	if err != nil {
		return err
	}
	file.Comments = "|user|"

	var over goatcounter.OverrideTranslations
	err = over.Get(r.Context(), true)
	if err != nil {
		return err
	}

	over = append(over, goatcounter.OverrideTranslation{
		Name:    args.Language + ".toml",
		Updated: time.Now().UTC().Round(time.Second).String(),
		File:    file,
	})

	err = over.Update(r.Context())
	if err != nil {
		return err
	}

	zhttp.Flash(w, "%q added", args.Language)
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

	var over goatcounter.OverrideTranslations
	err = over.Get(r.Context(), true)
	if err != nil {
		return err
	}

	found := -1
	for i, f := range over {
		if f.Name == file {
			found = i
			break
		}
	}
	if found == -1 { // No user copy yet, create one.
		orig, err := msgfile.ReadFile(goatcounter.Translations(r.Context()), file)
		if err != nil {
			return err
		}

		over = append(over, goatcounter.OverrideTranslation{
			Name:    file,
			Updated: time.Now().UTC().Round(time.Second).String(),
			File:    orig,
		})
		found = len(over) - 1
	}

	e := over[found].File.Strings[args.Entry.ID]
	e.Default = args.Entry.Default
	e.Zero = args.Entry.Zero
	e.One = args.Entry.One
	e.Two = args.Entry.Two
	e.Few = args.Entry.Few
	e.Many = args.Entry.Many
	over[found].File.Strings[args.Entry.ID] = e

	err = over.Update(r.Context())
	if err != nil {
		return err
	}

	return zhttp.JSON(w, map[string]interface{}{"success": true})
}

func (h i18n) set(w http.ResponseWriter, r *http.Request) error {
	var args struct {
		Language string `json:"language"`
	}
	_, err := zhttp.Decode(r, &args)
	if err != nil {
		return err
	}

	lang, _ := zfilepath.SplitExt(args.Language)

	u := User(r.Context())
	u.Settings.Language = lang
	err = u.Update(r.Context(), false)
	if err != nil {
		return err
	}

	zhttp.Flash(w, "language set to %q", lang)
	return zhttp.SeeOther(w, "/i18n")
}

func (h i18n) submit(w http.ResponseWriter, r *http.Request) error {
	file := chi.URLParam(r, "file")
	f, err := msgfile.ReadFile(goatcounter.Translations(r.Context()), file)
	if err != nil {
		return err
	}

	t, err := f.TOML()
	if err != nil {
		return err
	}

	msg := fmt.Sprintf("User: %d; language: %q\n\n%s", User(r.Context()).ID, file, t)

	go func() {
		err := blackmail.Send("GoatCounter translation updates",
			blackmail.From("", User(r.Context()).Email),
			blackmail.To("support@goatcounter.com"),
			blackmail.BodyText([]byte(msg)))
		if err != nil {
			zlog.Error(err)
		}
	}()

	zhttp.Flash(w, "email sent to support@goatcounter.com; I'll take a look as soon as possible.")
	return zhttp.SeeOther(w, "/i18n/"+file)
}

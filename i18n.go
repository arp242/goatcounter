package goatcounter

import (
	"context"
	"embed"
	"io/fs"
	"time"

	"github.com/BurntSushi/toml"
	"golang.org/x/text/language"
	"zgo.at/errors"
	"zgo.at/z18n"
	"zgo.at/z18n/msgfile"
	"zgo.at/zdb"
)

//go:embed i18n/*
var translations embed.FS

// TranslationFiles gets the translation messages.
var TranslationFiles = func() fs.FS {
	fsys, _ := fs.Sub(translations, "i18n")
	return fsys
}()

var Bundle = func() *z18n.Bundle {
	b, err := newBundle(TranslationFiles)
	if err != nil {
		panic(err)
	}
	return b
}()

var DefaultLocale = Bundle.Locale("en")

func newBundle(fsys fs.FS) (*z18n.Bundle, error) {
	b := z18n.NewBundle(language.MustParse("en-GB"))
	err := b.ReadMessagesDir(fsys, "*.toml")
	return b, err
}

type Translation msgfile.File

func (t *Translation) ByFilename(ctx context.Context, filename string) error {
	var s string
	err := zdb.Get(ctx, &s, `select value from store where key = ?`, "i18n-"+filename)
	if err != nil {
		return errors.Wrapf(err, "Translation.ByFilename(%q)", filename)
	}

	tt := msgfile.File(*t)
	_, err = toml.Decode(s, &tt)
	*t = Translation(tt)
	return errors.Wrapf(err, "Translation.ByFilename(%q)", filename)
}

func (t *Translation) Store(ctx context.Context, filename string) error {
	t.Modified = time.Now().UTC().Truncate(time.Second)
	tt, err := msgfile.File(*t).TOML()
	if err != nil {
		return errors.Wrapf(err, "Translation.Store(%q)", filename)
	}

	err = zdb.TX(ctx, func(ctx context.Context) error {
		k := "i18n-" + filename
		err := zdb.Exec(ctx, `delete from store where key = ?`, k)
		if err != nil {
			return err
		}
		err = zdb.Exec(ctx, `insert into store (key, value) values (?, ?)`, k, tt)
		return err
	})
	return errors.Wrapf(err, "Translation.Store(%q)", filename)
}

type Translations []Translation

func (t *Translations) List(ctx context.Context) error {
	var s []struct {
		Key   string `db:"key"`
		Value string `db:"value"`
	}
	err := zdb.Select(ctx, &s, `select * from store where key like 'i18n-%'`)
	if err != nil {
		return errors.Wrap(err, "Translation.List")
	}

	*t = make(Translations, len(s))
	deref := *t
	for i := range s {
		tt := msgfile.File(deref[i])
		_, err = toml.Decode(s[i].Value, &tt)
		deref[i] = Translation(tt)
		if err != nil {
			return errors.Wrap(err, "Translation.List")
		}
	}
	*t = deref
	return nil
}

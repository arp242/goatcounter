// Copyright © Martin Tournoij – This file is part of GoatCounter and published
// under the terms of a slightly modified EUPL v1.2 license, which can be found
// in the LICENSE file or at https://license.goatcounter.com

package goatcounter

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"testing/fstest"

	"github.com/BurntSushi/toml"
	"golang.org/x/text/language"
	"zgo.at/errors"
	"zgo.at/json"
	"zgo.at/z18n"
	"zgo.at/z18n/msgfile"
	"zgo.at/zdb"
	"zgo.at/zlog"
	"zgo.at/zstd/zfs"
)

// Translations contains all translation messages.
//go:embed i18n/*
var translations embed.FS

// Translations gets the translation messages; a user can have a local override,
// so we need to apply that per-user.
func Translations(ctx context.Context) fs.FS {
	builtin, _ := fs.Sub(translations, "i18n")
	if ctx == nil || GetUser(ctx) == nil {
		return builtin
	}

	var over OverrideTranslations
	err := over.Get(ctx, false)
	if err != nil {
		return builtin
	}

	mapfs := fstest.MapFS{}
	for _, o := range over {
		t, _ := o.File.TOML()
		mapfs[o.Name] = &fstest.MapFile{Data: []byte(t)}
	}
	return zfs.NewOverlayFS(builtin, mapfs)
}

var defaultBundle = func() *z18n.Bundle {
	b, err := newBundle(Translations(context.TODO()))
	if err != nil {
		panic(err)
	}
	return b
}()

func GetBundle(ctx context.Context) *z18n.Bundle {
	if ctx == nil || GetUser(ctx) == nil {
		return defaultBundle
	}

	var over OverrideTranslations
	err := over.Get(ctx, false)
	if err != nil {
		if !zdb.ErrNoRows(err) {
			zlog.Error(err)
		}
		return defaultBundle
	}

	b, err := newBundle(Translations(ctx))
	if err != nil {
		zlog.Error(err)
		return defaultBundle
	}

	return b
}

func newBundle(fsys fs.FS) (*z18n.Bundle, error) {
	b := z18n.NewBundle(language.MustParse("en-GB"))
	err := b.ReadMessagesDir(fsys, "*.toml")
	return b, err
}

type OverrideTranslation struct {
	Name    string       `json:"name"`
	Updated string       `json:"updated"`
	File    msgfile.File `json:"file"`
}

type OverrideTranslations []OverrideTranslation

func (OverrideTranslations) Key(ctx context.Context) string {
	return fmt.Sprintf("i18n-%d", MustGetUser(ctx).ID)
}

type wrap []wrapF
type wrapF struct{ Name, Updated, TOML string }

func (o OverrideTranslations) encode() (string, error) {
	var w wrap
	for _, oo := range o {
		t, err := oo.File.TOML()
		if err != nil {
			return "", err
		}
		w = append(w, wrapF{
			Name:    oo.Name,
			Updated: oo.Updated,
			TOML:    t,
		})
	}
	j, err := json.MarshalIndent(w, "", "    ")
	return string(j), err
}

func (o *OverrideTranslations) decode(data string) error {
	var w wrap
	err := json.Unmarshal([]byte(data), &w)
	if err != nil {
		return err
	}

	oo := make(OverrideTranslations, 0, len(w))
	for _, ww := range w {
		var f msgfile.File
		_, err := toml.Decode(ww.TOML, &f)
		if err != nil {
			return err
		}

		oo = append(oo, OverrideTranslation{
			Name:    ww.Name,
			Updated: ww.Updated,
			File:    f,
		})
	}

	*o = oo
	return nil
}

// Insert new.
func (o *OverrideTranslations) Insert(ctx context.Context) error {
	t, err := o.encode()
	if err != nil {
		return errors.Wrap(err, "OverrideTranslations.Insert")
	}

	err = zdb.Exec(ctx, `insert into store (key, value) values (?, ?)`, o.Key(ctx), t)
	if err != nil {
		return errors.Wrap(err, "OverrideTranslations.Insert")
	}

	cacheI18n(ctx).Delete(o.Key(ctx))
	return nil
}

func (o *OverrideTranslations) Update(ctx context.Context) error {
	t, err := o.encode()
	if err != nil {
		return errors.Wrap(err, "OverrideTranslations.Update")
	}

	err = zdb.Exec(ctx, `update store set value=? where key=?`, t, o.Key(ctx))
	if err != nil {
		return errors.Wrap(err, "OverrideTranslations.Update")
	}

	cacheI18n(ctx).Delete(o.Key(ctx))
	return nil
}

func (o *OverrideTranslations) Get(ctx context.Context, insert bool) error {
	if oo, ok := cacheI18n(ctx).Get(o.Key(ctx)); ok {
		*o = *oo.(*OverrideTranslations)
		return nil
	}

	var data []byte
	err := zdb.Get(ctx, &data, `select value from store where key = ?`, o.Key(ctx))
	if err != nil {
		if insert && zdb.ErrNoRows(err) {
			*o = OverrideTranslations{}
			return o.Insert(ctx)
		}
		return errors.Wrap(err, "OverrideTranslations.Get")
	}

	err = o.decode(string(data))
	if err != nil {
		return errors.Wrap(err, "OverrideTranslations.List")
	}

	cacheI18n(ctx).SetDefault(o.Key(ctx), o)
	return nil
}

package goatcounter_test

import (
	"io/fs"
	"os"
	"strings"
	"testing"

	"zgo.at/errors"
	. "zgo.at/goatcounter/v2"
	"zgo.at/goatcounter/v2/gctest"
	"zgo.at/zstd/zgo"
	"zgo.at/zstd/ztype"
	"zgo.at/ztpl"
)

func TestTpl(t *testing.T) {
	ctx := gctest.Context(nil)
	site := Site{Code: "example"}
	user := User{Email: "a@example.com", EmailToken: ztype.Ptr("T-EMAIL"), LoginRequest: ztype.Ptr("T-LOGIN-REQ")}

	files, _ := fs.Sub(os.DirFS(zgo.ModuleRoot()), "tpl")
	err := ztpl.Init(files)
	if err != nil {
		t.Fatal(err)
	}

	errs := errors.NewGroup(4)
	errs.Append(errors.New("err: <1>"))
	errs.Append(errors.New("err: <2>"))
	errs.Append(errors.New("err: <3>"))
	errs.Append(errors.New("err: <4>"))
	errs.Append(errors.New("err: <5>"))

	tests := []struct {
		t interface{ Render() ([]byte, error) }
	}{
		{TplEmailWelcome{ctx, site, user, "count.example.com"}},
		{TplEmailForgotSite{ctx, []Site{site}, "test@example.com"}},
		{TplEmailForgotSite{ctx, []Site{}, "test@example.com"}},
		{TplEmailPasswordReset{ctx, site, user}},
		{TplEmailVerify{ctx, site, user}},
		{TplEmailImportError{ctx, errors.Unwrap(errors.New("oh noes"))}},
		{TplEmailImportDone{ctx, site, 42, errors.NewGroup(10)}},
		{TplEmailImportDone{ctx, site, 42, errs}},
		{TplEmailAddUser{ctx, site, user, "foo@example.com"}},

		{TplEmailExportDone{ctx, site, user, Export{
			ID:        2,
			NumRows:   ztype.Ptr(42),
			Size:      ztype.Ptr("42"),
			LastHitID: ztype.Ptr(HitID(642051)),
			Hash:      ztype.Ptr("sha256-AAA"),
		}}},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			got, err := tt.t.Render()
			if err != nil {
				t.Fatal(err)
			}

			want := "Cheers,\nMartin\n"
			if !strings.Contains(string(got), want) {
				t.Errorf("didn't contain %q", want)
			}

			t.Log("\n" + string(got))
		})
	}
}

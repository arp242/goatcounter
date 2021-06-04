package z18n

import (
	"testing"

	"golang.org/x/text/language"
)

func mkbundle() *Bundle {
	b := NewBundle(language.English)
	b.AddMessages(language.English, map[string]Msg{
		"hello":     Msg{other: "Hello"},
		"hello-loc": Msg{other: "Hello, %(loc)!"},
		"btn":       Msg{other: "%[btn Button]"},
		"btn2":      Msg{other: "%[btn Button %(x)]"},
		"btn3":      Msg{other: "%[btn Send]"},
	})
	b.AddMessages(language.Dutch, map[string]Msg{
		"hello":     Msg{other: "Hallo"},
		"hello-loc": Msg{other: "Hallo, %(loc)!"},
		"btn":       Msg{other: "%[btn Knop]"},
		"btn2":      Msg{other: "%[btn Knop %(x)]"},
		"btn3":      Msg{other: "%[btn Verstuur]"},
	})
	return b
}

func TestT(t *testing.T) {
	tests := []struct {
		id     string
		data   []interface{}
		wantEN string
		wantNL string
	}{
		{"hello", nil, "Hello", "Hallo"},
		{"hello-loc", []interface{}{"z18n"}, "Hello, z18n!", "Hallo, z18n!"},

		{"id|Default msg", nil, "Default msg", "Default msg"},
		{"unknown", nil, "unknown", "unknown"},

		// Placeholders
		{"|%(var)", []interface{}{"xx"}, "xx", "xx"},
		{"|$%(var)", []interface{}{"xx"}, "$xx", "$xx"},
		{"|%%(var)", []interface{}{"xx"}, "%(var)", "%(var)"},
		{"%(var)", nil, "%(var)", "%(var)"},

		// Buttons
		{"btn|%[btn Button]", []interface{}{Tag("a", `href="/foo"`)}, `<a href="/foo">Button</a>`, `<a href="/foo">Knop</a>`},
		{"btn2|%[btn Button %(var)]", []interface{}{Tag("a", `href="/foo"`), "X"}, `<a href="/foo">Button X</a>`, `<a href="/foo">Knop X</a>`},
		{"btn3", []interface{}{TagNone()}, `Send`, `Verstuur`},

		// Plural.
		{"hello", []interface{}{N(5)}, "Hello", "Hallo"},
		{"hello-loc", []interface{}{N(5), "z18n"}, "Hello, z18n!", "Hallo, z18n!"},

		//{"hello-loc", nil, "Hello", "Hallo"}, // TODO: panic
		//{"hello", []interface{}{"hmm"}, "Hello", "Hallo"}, // TODO: don't silently ignore extra data
	}

	b := mkbundle()

	en := b.Locale("en_UK")
	nl := b.Locale("nl_NL")

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			haveEN := en.T(tt.id, tt.data...)
			if haveEN != tt.wantEN {
				t.Errorf("English wrong\nhave: %s\nwant: %s", haveEN, tt.wantEN)
			}

			haveNL := nl.T(tt.id, tt.data...)
			if haveNL != tt.wantNL {
				t.Errorf("Dutch wrong\nhave: %s\nwant: %s", haveNL, tt.wantNL)
			}
		})
	}
}

func BenchmarkNew(b *testing.B) {
	b.ReportAllocs()
	for n := 0; n < b.N; n++ {
		bundle := mkbundle()
		_ = bundle.Locale("nl_NL")
	}
}

func BenchmarkT(b *testing.B) {
	bundle := mkbundle()
	nl := bundle.Locale("nl_NL")

	b.ReportAllocs()
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		_ = nl.T("btn2", "a")
	}
}

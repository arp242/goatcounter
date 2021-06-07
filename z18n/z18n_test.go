package z18n

import (
	"testing"

	"golang.org/x/text/language"
)

func mkbundle() *Bundle {
	b := NewBundle(language.English)
	b.AddMessages(language.English, map[string]Msg{
		"hello":     Msg{Other: "Hello"},
		"hello-loc": Msg{Other: "Hello, %(loc)!"},
		"btn":       Msg{Other: "%[btn Button]"},
		"btn2":      Msg{Other: "Hello %[btn1 Button], %[btn2 another] XX"},
		"btn3":      Msg{Other: "%[btn Button %(var)]"},
		"btn4":      Msg{Other: "%[btn Send]"},
	})
	b.AddMessages(language.Dutch, map[string]Msg{
		"hello":     Msg{Other: "Hallo"},
		"hello-loc": Msg{Other: "Hallo, %(loc)!"},
		"btn":       Msg{Other: "%[btn Knop]"},
		"btn2":      Msg{Other: "Hallo %[btn1 Knop], %[btn2 nog een] XX"},
		"btn3":      Msg{Other: "%[btn Knop %(var)]"},
		"btn4":      Msg{Other: "%[btn Verstuur]"},
	})
	return b
}

func TestT(t *testing.T) {
	tests := []struct {
		name   string
		id     string
		data   []interface{}
		wantEN string
		wantNL string
	}{
		{"empty string",
			"", nil, "", ""},
		{"basic string",
			"hello", nil, "Hello", "Hallo"},
		{"default message",
			"id|Default msg", nil, "Default msg", "Default msg"},
		{"unknown key",
			"unknown", nil, "unknown", "unknown"},

		// Variables
		{"variable",
			"hello-loc", []interface{}{"z18n"}, "Hello, z18n!", "Hallo, z18n!"},
		{"variable in default msg",
			"|%(var)", []interface{}{"xx"}, "xx", "xx"},
		{"variable in id",
			"%(var)", nil, "%(var)", "%(var)"},

		{"two variables",
			"|%(var) %(bar)", []interface{}{P{"var": "xx", "bar": "yy"}}, "xx yy", "xx yy"},

		// HTML
		{"html",
			"btn|%[btn Button]", []interface{}{Tag("a", `href="/foo"`)},
			`<a href="/foo">Button</a>`, `<a href="/foo">Knop</a>`},
		{"html two", "btn2", []interface{}{P{
			"btn1": Tag("a", `href="/btn1"`),
			"btn2": Tag("a", `href="/btn2"`),
		}},
			`Hello <a href="/btn1">Button</a>, <a href="/btn2">another</a> XX`,
			`Hallo <a href="/btn1">Knop</a>, <a href="/btn2">nog een</a> XX`,
		},
		{"var inside html", "btn3|%[btn Button %(var)]", []interface{}{P{
			"var": "X",
			"btn": Tag("a", `href="/foo"`),
		}},
			`<a href="/foo">Button X</a>`, `<a href="/foo">Knop X</a>`},

		{"tag none", "btn4", []interface{}{TagNone()}, `Send`, `Verstuur`},

		// Plural.
		{"plural", "hello", []interface{}{N(5)}, "Hello", "Hallo"},
		{"plural", "hello-loc", []interface{}{N(5), "z18n"}, "Hello, z18n!", "Hallo, z18n!"},

		// Errors
		{"extra params", "hello", []interface{}{"X"}, "Hello", "Hallo"},
		{"no vars", "hello-loc", nil, "Hello, %(z18n ERROR: no value for loc)!", "Hallo, %(z18n ERROR: no value for loc)!"},
	}

	b := mkbundle()

	en := b.Locale("en_UK")
	nl := b.Locale("nl_NL")

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
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

	b.ResetTimer()

	b.Run("string", func(b *testing.B) {
		b.ReportAllocs()
		for n := 0; n < b.N; n++ {
			_ = nl.T("hello", "a")
		}
	})
	b.Run("one-variable", func(b *testing.B) {
		b.ReportAllocs()
		for n := 0; n < b.N; n++ {
			_ = nl.T("hello-loc", "a")
		}
	})
	b.Run("one-tag", func(b *testing.B) {
		b.ReportAllocs()
		for n := 0; n < b.N; n++ {
			_ = nl.T("btn", Tag("a", `href="/foo"`))
		}
	})
}

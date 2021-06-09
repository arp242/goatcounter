package z18n

import (
	"fmt"
	"testing"
	"time"

	"golang.org/x/text/language"
)

func mkbundle() *Bundle {
	b := NewBundle(language.MustParse("en-GB"))
	b.AddMessages(language.MustParse("en-GB"), map[string]Msg{
		"hello":     Msg{Default: "Hello"},
		"hello-loc": Msg{Default: "Hello, %(loc)!"},
		"btn":       Msg{Default: "%[btn Button]"},
		"btn2":      Msg{Default: "Hello %[btn1 Button], %[btn2 another] XX"},
		"btn3":      Msg{Default: "%[btn Button %(var)]"},
		"btn4":      Msg{Default: "%[btn Send]"},
		"ants!": Msg{
			One:     "Help, I've got an ant in my trousers!",
			Default: "Help, I've got %(n) ants in my trousers!",
		},
	})
	b.AddMessages(language.MustParse("nl-NL"), map[string]Msg{
		"hello":     Msg{Default: "Hallo"},
		"hello-loc": Msg{Default: "Hallo, %(loc)!"},
		"btn":       Msg{Default: "%[btn Knop]"},
		"btn2":      Msg{Default: "Hallo %[btn1 Knop], %[btn2 nog een] XX"},
		"btn3":      Msg{Default: "%[btn Knop %(var)]"},
		"btn4":      Msg{Default: "%[btn Verstuur]"},
		"ants!": Msg{
			One:     "Help, ik heb een mier in m'n broek!",
			Default: "Help, ik heb %(n) mieren in m'n broek!",
		},
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

		{"ants!", "ants!", []interface{}{N(0)}, "Help, I've got 0 ants in my trousers!", "Help, ik heb 0 mieren in m'n broek!"},
		{"ants!", "ants!", []interface{}{N(1)}, "Help, I've got an ant in my trousers!", "Help, ik heb een mier in m'n broek!"},
		{"ants!", "ants!", []interface{}{N(2)}, "Help, I've got 2 ants in my trousers!", "Help, ik heb 2 mieren in m'n broek!"},

		// TODO: error reporting could be a bit better on this. Also have z18n
		// CLI catch this.
		{"ants!", "ants!", []interface{}{},
			"Help, I've got %(z18n ERROR: no value for n) ants in my trousers!",
			"Help, ik heb %(z18n ERROR: no value for n) mieren in m'n broek!"},
		{"plural", "hello", []interface{}{N(1)},
			"%(z18n ERROR: plural form one is empty for en-GB)",
			"%(z18n ERROR: plural form one is empty for nl-NL)"},

		// Errors
		{"extra params", "hello", []interface{}{"X"}, "Hello", "Hallo"},
		{"no vars", "hello-loc", nil, "Hello, %(z18n ERROR: no value for loc)!", "Hallo, %(z18n ERROR: no value for loc)!"},
	}

	b := mkbundle()

	en := b.Locale("en-GB")
	nl := b.Locale("nl-NL")

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

func TestL10n(t *testing.T) {
	tests := []struct {
		in string
	}{
		{""},
	}

	b := NewBundle(language.MustParse("en-US"))
	b.AddMessages(language.MustParse("en-US"), map[string]Msg{
		"order": Msg{
			Default: "Your order of %(n) gophers will arrive on %(d).",
		},
	})
	b.AddMessages(language.MustParse("en-NZ"), map[string]Msg{
		"order": Msg{
			Default: "Your order of %(n) gophers will arrive on %(d).",
		},
	})
	b.AddMessages(language.MustParse("nl-NL"), map[string]Msg{
		"order": Msg{
			Default: "Uw bestelling van %(n) gophers wordt op %(d) geleverd",
		},
	})

	lUS := b.Locale("en-US")
	lNZ := b.Locale("en-NZ")
	lNL := b.Locale("nl-NL")

	data := P{
		"n": 153_416_164_1,
		"d": time.Date(2021, 06, 18, 0, 0, 0, 0, time.UTC),
	}
	fmt.Println(lUS.T("order", data))
	fmt.Println(lNZ.T("order", data))
	fmt.Println(lNL.T("order", data))

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			_ = tt

			// if have != tt.want {
			// 	t.Errorf("\nhave: %q\nwant: %q", have, tt.want)
			// }
			// if !reflect.DeepEqual(have, tt.want) {
			// 	t.Errorf("\nhave: %#v\nwant: %#v", have, tt.want)
			// }
			// if d := ztest.Diff(have, tt.want); d != "" {
			// 	t.Errorf(d)
			// }
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

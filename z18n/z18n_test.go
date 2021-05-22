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
		"btn":       Msg{other: "[[Button]]"},
		"btn2":      Msg{other: "[[Button %(x)]]"},
	})
	b.AddMessages(language.Dutch, map[string]Msg{
		"hello":     Msg{other: "Hallo"},
		"hello-loc": Msg{other: "Hallo, %(loc)!"},
		"btn":       Msg{other: "[[Knop]]"},
		"btn2":      Msg{other: "[[Knop %(x)]]"},
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
		{"btn|[[Button]]", nil, "Button", "Knop"},
		{"btn2|[[Button %(x)]]", []interface{}{"XXX"}, "Button XXX", "Knop XXX"},

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
				t.Errorf("English wrong\nhave: %q\nwant: %q", haveEN, tt.wantEN)
			}

			haveNL := nl.T(tt.id, tt.data...)
			if haveNL != tt.wantNL {
				t.Errorf("Dutch wrong\nhave: %q\nwant: %q", haveNL, tt.wantNL)
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

package z18n

import (
	"reflect"
	"time"

	"github.com/goodsign/monday"
	"golang.org/x/text/language"
	"golang.org/x/text/message"
)

// Convert and "simplify" a type:
//
//    everything else       No conversation, return as-is.
//    bool                  reflect.Bool
//    complex*              reflect.Complex128
//    int*, uint*, float*   reflect.Float64; as floats are reliable for natural
//                          numbers up to ~9e15 (9,007,199 billion) just
//                          converting it to a float will be fine in most use
//                          cases.
//    string, []byte        reflect.String; note this also matches []uint8 and []uint32,
//            []rune        as byte and rune are just aliases for that with no way to
//            fmt.Stringer  distinguish between the two.
//
// The practical value of this is that it makes it a lot easier to deal with
// different types:
//
// TODO: move to zstd/zreflect
func Convert(value interface{}) (reflect.Value, reflect.Kind) {
	v := reflect.ValueOf(value)
	// if v.Type().Implements(reflect.TypeOf((*fmt.Stringer)(nil)).Elem()) {
	// 	v = v.MethodByName("String").Call(nil)[0]
	// 	return v, reflect.String
	// }

top:
	switch v.Kind() {
	default:
		return v, v.Kind()
	case reflect.Ptr:
		v = v.Elem()
		goto top

	case reflect.Bool:
		return v, reflect.Bool
	case reflect.Complex64, reflect.Complex128:
		return v.Convert(reflect.TypeOf(complex128(0))), reflect.Complex128
	case reflect.String:
		return v, reflect.String

	case reflect.SliceOf(reflect.TypeOf([]byte{})).Kind(): // []uint8 matches this as well.
		return v.Convert(reflect.TypeOf("")), reflect.String
	case reflect.SliceOf(reflect.TypeOf([]rune{})).Kind(): // []uint32 matches this as well.
		return v.Convert(reflect.TypeOf("")), reflect.String

	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64:
		return v.Convert(reflect.TypeOf(float64(0))), reflect.Float64
	}
}

// TODO: some syntax to format as float with precision?
//
// TODO: finish this; the date information isn't readily accesible, and I'm not
// even a huge fan of how message.NewPrinter() etc. works either.
func l10n(tag language.Tag, v interface{}) string {
	switch vv, t := Convert(v); t {
	case reflect.Float64:
		return message.NewPrinter(tag).Sprintf("%.0f", vv.Float())
	case reflect.String:
		return vv.String()
	default:
		if vv.Type() == reflect.TypeOf(time.Time{}) {
			t := vv.Interface().(time.Time)
			// TODO: the monday package is okay for now, but it's not really a great
			// one. But it's the only one I could find :-/
			// The data isn't based on the CLDR and far from complete, only
			// supports Gregorian calendar, and the API is pretty awkward.
			//
			// Also, there is no way to format just times :-/
			//
			// Oh, and the Dutch date format isn't even correct as it uses "06"
			// instead of "2006"...
			//
			// Actually, this package won't really do at all... But keep it here
			// until I have time to write a replacement.
			l := mondayMap(tag)

			// TODO: support formats:
			// date            2006-01-02
			// time            15:16
			// date-long       Jan 02, 2006
			// datetime        2006-01-02 15:16
			// datetime-long   Monday, January 2, 2006
			return monday.Format(t, monday.DateTimeFormatsByLocale[l], l)
		}

		return vv.String()
	}
}

func mondayMap(t language.Tag) monday.Locale {
	found, ok := map[language.Tag]monday.Locale{
		language.MustParse("bg-BG"): monday.LocaleBgBG, // Bulgarian (Bulgaria)
		language.MustParse("ca-ES"): monday.LocaleCaES, // Catalan (Spain)
		language.MustParse("cs-CZ"): monday.LocaleCsCZ, // Czech (Czech Republic)
		language.MustParse("da-DK"): monday.LocaleDaDK, // Danish (Denmark)
		language.MustParse("de-DE"): monday.LocaleDeDE, // German (Germany)
		language.MustParse("el-GR"): monday.LocaleElGR, // Greek (Greece)
		language.MustParse("en-GB"): monday.LocaleEnGB, // English (United Kingdom)
		language.MustParse("en-US"): monday.LocaleEnUS, // English (United States)
		language.MustParse("es-ES"): monday.LocaleEsES, // Spanish (Spain)
		language.MustParse("fi-FI"): monday.LocaleFiFI, // Finnish (Finland)
		language.MustParse("fr-CA"): monday.LocaleFrCA, // French (Canada)
		language.MustParse("fr-FR"): monday.LocaleFrFR, // French (France)
		language.MustParse("fr-GF"): monday.LocaleFrGF, // French (French Guiana)
		language.MustParse("fr-GP"): monday.LocaleFrGP, // French (Guadeloupe)
		language.MustParse("fr-LU"): monday.LocaleFrLU, // French (Luxembourg)
		language.MustParse("fr-MQ"): monday.LocaleFrMQ, // French (Martinique)
		language.MustParse("fr-RE"): monday.LocaleFrRE, // French (Reunion)
		language.MustParse("hu-HU"): monday.LocaleHuHU, // Hungarian (Hungary)
		language.MustParse("id-ID"): monday.LocaleIdID, // Indonesian (Indonesia)
		language.MustParse("it-IT"): monday.LocaleItIT, // Italian (Italy)
		language.MustParse("ja-JP"): monday.LocaleJaJP, // Japanese (Japan)
		language.MustParse("ko-KR"): monday.LocaleKoKR, // Korean (Korea)
		language.MustParse("nb-NO"): monday.LocaleNbNO, // Norwegian Bokmål (Norway)
		language.MustParse("nl-BE"): monday.LocaleNlBE, // Dutch (Belgium)
		language.MustParse("nl-NL"): monday.LocaleNlNL, // Dutch (Netherlands)
		language.MustParse("nn-NO"): monday.LocaleNnNO, // Norwegian Nynorsk (Norway)
		language.MustParse("pl-PL"): monday.LocalePlPL, // Polish (Poland)
		language.MustParse("pt-BR"): monday.LocalePtBR, // Portuguese (Brazil)
		language.MustParse("pt-PT"): monday.LocalePtPT, // Portuguese (Portugal)
		language.MustParse("ro-RO"): monday.LocaleRoRO, // Romanian (Romania)
		language.MustParse("ru-RU"): monday.LocaleRuRU, // Russian (Russia)
		language.MustParse("sl-SI"): monday.LocaleSlSI, // Slovenian (Slovenia)
		language.MustParse("sv-SE"): monday.LocaleSvSE, // Swedish (Sweden)
		language.MustParse("tr-TR"): monday.LocaleTrTR, // Turkish (Turkey)
		language.MustParse("uk-UA"): monday.LocaleUkUA, // Ukrainian (Ukraine)
		language.MustParse("zh-CN"): monday.LocaleZhCN, // Chinese (Mainland)
		language.MustParse("zh-HK"): monday.LocaleZhHK, // Chinese (Hong Kong)
		language.MustParse("zh-TW"): monday.LocaleZhTW, // Chinese (Taiwan)
	}[t]
	if ok {
		return found
	}

	found, ok = map[language.Tag]monday.Locale{
		language.MustParse("bg"): monday.LocaleBgBG, // Bulgarian (Bulgaria)
		language.MustParse("ca"): monday.LocaleCaES, // Catalan (Spain)
		language.MustParse("cs"): monday.LocaleCsCZ, // Czech (Czech Republic)
		language.MustParse("da"): monday.LocaleDaDK, // Danish (Denmark)
		language.MustParse("de"): monday.LocaleDeDE, // German (Germany)
		language.MustParse("el"): monday.LocaleElGR, // Greek (Greece)
		language.MustParse("en"): monday.LocaleEnGB, // English (United Kingdom)
		language.MustParse("es"): monday.LocaleEsES, // Spanish (Spain)
		language.MustParse("fi"): monday.LocaleFiFI, // Finnish (Finland)
		language.MustParse("fr"): monday.LocaleFrFR, // French (France)
		language.MustParse("hu"): monday.LocaleHuHU, // Hungarian (Hungary)
		language.MustParse("id"): monday.LocaleIdID, // Indonesian (Indonesia)
		language.MustParse("it"): monday.LocaleItIT, // Italian (Italy)
		language.MustParse("ja"): monday.LocaleJaJP, // Japanese (Japan)
		language.MustParse("ko"): monday.LocaleKoKR, // Korean (Korea)
		language.MustParse("nb"): monday.LocaleNbNO, // Norwegian Bokmål (Norway)
		language.MustParse("nl"): monday.LocaleNlNL, // Dutch (Netherlands)
		language.MustParse("nn"): monday.LocaleNnNO, // Norwegian Nynorsk (Norway)
		language.MustParse("pl"): monday.LocalePlPL, // Polish (Poland)
		language.MustParse("pt"): monday.LocalePtBR, // Portuguese (Brazil)
		language.MustParse("ro"): monday.LocaleRoRO, // Romanian (Romania)
		language.MustParse("ru"): monday.LocaleRuRU, // Russian (Russia)
		language.MustParse("sl"): monday.LocaleSlSI, // Slovenian (Slovenia)
		language.MustParse("sv"): monday.LocaleSvSE, // Swedish (Sweden)
		language.MustParse("tr"): monday.LocaleTrTR, // Turkish (Turkey)
		language.MustParse("uk"): monday.LocaleUkUA, // Ukrainian (Ukraine)
		language.MustParse("zh"): monday.LocaleZhCN, // Chinese (Mainland)
	}[t.Parent()]
	if ok {
		return found
	}

	return monday.LocaleEnGB
}

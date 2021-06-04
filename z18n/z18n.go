// Package z18n adds support for translations.
package z18n

import (
	"context"
	"fmt"
	"strings"

	"golang.org/x/text/language"
	"zgo.at/goatcounter/z18n/plural"
	"zgo.at/zstd/zstring"
)

// TODO: Add support for localizing numbers, dates.

type (
	// Bundle is a "bundle" of all translations and localisations.
	Bundle struct {
		defaultLang language.Tag
		tags        []language.Tag
		matcher     language.Matcher
		pluralRules plural.Rules

		// We need to store the messages on the Bundle; we can't store that on
		// the Locale as a single Locale may have multiple languages associated
		// with it.
		//
		// For example, someone may prefer Dutch, falling back to German
		// (instead of the default English). This is also how we can do
		// "inheritance" by the way: e.g. have [en-UK en-US] and only have
		// messages in en-US that need to be overridden.
		messages map[language.Tag]map[string]Msg
	}

	// Locale is a single localisation.
	Locale struct {
		bundle *Bundle
		tags   []language.Tag
	}

	// Msg is a localized message.
	Msg struct {
		id     string
		vars   []interface{}
		tags   []Tagger
		plural Plural
		other  string
	}
)

// NewBundle creates a new bundle of languages, falling back to defaultLang if a
// chosen language doesn't exist.
func NewBundle(defaultLang language.Tag) *Bundle {
	b := &Bundle{
		defaultLang: defaultLang,
		tags:        make([]language.Tag, 0, 8),
		messages:    make(map[language.Tag]map[string]Msg),
		pluralRules: plural.DefaultRules(),
	}
	b.addTag(defaultLang)
	return b
}

// AddMessages adds new messages for this language.
func (b *Bundle) AddMessages(l language.Tag, msg map[string]Msg) {
	if b.messages[l] == nil {
		b.addTag(l)
		b.messages[l] = msg
		return
	}

	for k, v := range msg {
		b.messages[l][k] = v
	}
}

// Locale gets the first matching locale for the list of languages.
func (b *Bundle) Locale(langs ...string) *Locale {
	l := &Locale{bundle: b, tags: make([]language.Tag, 0, len(langs)+1)}
	for _, lang := range langs {
		t, _, err := language.ParseAcceptLanguage(lang)
		if err != nil {
			continue
		}
		l.tags = append(l.tags, t...)
	}
	l.tags = append(l.tags, b.defaultLang)
	return l
}

func (b *Bundle) addTag(tag language.Tag) {
	for _, t := range b.tags {
		if t == tag {
			return
		}
	}
	b.tags = append(b.tags, tag)
	b.matcher = language.NewMatcher(b.tags)
}

// T translates a message for this locale.
//
// It will return the message in the bundler's defaultLag if the message is not
// translated in this language (yet).
//
// The ID can contain any character except a |. Everything after the first | is
// used as the default message.
//
// Example:
//
//   T("asd")                                          Just ID
//   T("asd", email)                                   With ID and params
//   T("asd|default msg %(email)", email)              With default message and params.
//   T("asd|default msg: %(email)", email, z18n.N(5))  Apply pluralisation.
func (l Locale) T(id string, data ...interface{}) string {
	def := ""
	if p := strings.Index(id, "|"); p > -1 {
		id, def = id[:p], id[p+1:]
	}

	var (
		pl   Plural
		vars []interface{}
		tags []Tagger
	)
	for i, d := range data {
		if p, ok := d.(Plural); ok {
			pl = p
			data = append(data[:i], data[i+1:]...)
		} else if t, ok := d.(Tagger); ok {
			tags = append(tags, t)
		} else {
			vars = append(vars, d)
		}
	}

	_, i, _ := l.bundle.matcher.Match(l.tags...)
	tag := l.bundle.tags[i]

	m, ok := l.bundle.messages[tag]
	if ok {
		msg, ok := m[id]
		if ok {
			msg.id = id
			msg.vars = vars
			msg.tags = tags
			msg.plural = pl
			return msg.String()
		}
	}

	return Msg{
		id:     id,
		other:  def,
		tags:   tags,
		vars:   vars,
		plural: pl,
	}.String()
}

// Plural signals to T that this parameter is used to pluralize the string,
// rather than a data parameter.
type Plural int

// N returns a plural of n.
func N(n int) Plural { return Plural(n) }

var funcmap = map[string]func(string) string{
	"lower": strings.ToLower,
}

//   %(word)                 Replace
//   %%(word)                Literal "%(word)"
//   %(word ucfirst)         Apply function
//   %(word lower ucfirst)   Apply two functions
//
//   %[text]
//
// Functions are mainly useful in languages that require some capitalisation,
// e.g. in German most proper nouns are capitalized. This allows varying this
// per translation.
//
// TODO: don't use positional, find by name instead.
func (m Msg) tpl(str string) string {
	str = zstring.ReplacePairs(str, "%[", "]", func(i int, match string) string {
		if i > len(m.tags) {
			return fmt.Sprintf("z18n: too many variables (%d)", i)
		}

		sp := strings.IndexRune(match, ' ')
		varname, text := strings.TrimSpace(match[2:sp]), strings.TrimSpace(match[sp+1:len(match)-1])
		_ = varname

		v := m.tags[i]
		return v.Open() + text + v.Close()
	})

	str = zstring.ReplacePairs(str, "%(", ")", func(i int, match string) string {
		if i > len(m.vars) {
			return fmt.Sprintf("z18n: too many variables (%d)", i)
		}

		v := zstring.String(m.vars[i])
		if !strings.Contains(match, " ") {
			return v
		}

		funs := strings.Fields(match)[1:]
		for _, f := range funs {
			fn, ok := funcmap[f]
			if ok {
				v = fn(v)
			}
		}
		return v
	})

	return str
}

// String displays this string as "other".
func (m Msg) String() string {
	// TODO: implement plurals.

	if m.other != "" {
		return m.tpl(m.other)
	}
	return m.id
}

var ctxkey = &struct{}{}

// With returns a copy of the context with the Locale as a value.
func With(ctx context.Context, l *Locale) context.Context {
	return context.WithValue(ctx, ctxkey, l)
}

// Get the Locale value from the context.
func Get(ctx context.Context) *Locale {
	l, _ := ctx.Value(ctxkey).(*Locale)
	return l
}

// T translates a string, like Locale.T. the Locale is fetched from the context.
func T(ctx context.Context, id string, data ...interface{}) string {
	l := Get(ctx)
	if l == nil {
		return ""
	}
	return l.T(id, data...)
}

// Package z18n adds support for translations.
package z18n

import (
	"context"
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
		ID     string
		Plural Plural
		Other  string
		data   P
		oneVar bool
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

type P map[string]interface{}

// T translates a message for this locale.
//
// It will return the message in the bundler's defaultLang if the message is not
// translated in this language (yet).
//
// The ID can contain any character except a |. Everything after the first | is
// used as the default message.
//
// Variables can be inserted as %(varname), and text can be wrapped in HTML tags
// with %[varname translated text]. Wrapping in HTML requires passing a Tagger
// interface (such as Tag).
//
// Pass N() as any argument to apply pluralisation.
func (l Locale) T(id string, data ...interface{}) string {
	def := ""
	if p := strings.Index(id, "|"); p > -1 {
		id, def = id[:p], id[p+1:]
	}

	// data can contain be any of three things:
	//   - A Plural, *AND*
	//   - A P for mapped variables, *OR*
	//   - Anything else for a single variable.
	// The z18n tool checks the parameters, so we don't need to do a lot of that
	// here and we can be a bit relaxed.
	var (
		pl     Plural
		params P
		oneVar bool
	)
	for _, d := range data {
		if p, ok := d.(Plural); ok {
			pl = p
		} else if p, ok := d.(P); ok {
			params = p
		} else {
			oneVar, params = true, P{"": d}
		}
	}

	_, i, _ := l.bundle.matcher.Match(l.tags...)
	tag := l.bundle.tags[i]

	m, ok := l.bundle.messages[tag]
	if ok {
		msg, ok := m[id]
		if ok {
			msg.ID = id
			msg.data = params
			msg.oneVar = oneVar
			msg.Plural = pl
			return msg.String()
		}
	}

	return Msg{
		ID:     id,
		Other:  def,
		data:   params,
		oneVar: oneVar,
		Plural: pl,
	}.String()
}

// Plural signals to T that this parameter is used to pluralize the string,
// rather than a data parameter.
type Plural int

// N returns a plural of n.
func N(n int) Plural { return Plural(n) }

var funcmap = map[string]func(string) string{
	"lower":       strings.ToLower,
	"upper":       strings.ToUpper,
	"upper_first": zstring.UpperFirst,
}

func (m Msg) tpl(str string) string {
	var (
		tags  = zstring.IndexPairs(str, "%[", "]")
		vars  = zstring.IndexPairs(str, "%(", ")")
		total = len(tags) + len(vars)
	)
	if total == 0 {
		return str
	}

	str = m.tplTags(str, tags)

	// We need to check this again, as the indexes probably changed.
	// TODO: this can be more efficient.
	vars = zstring.IndexPairs(str, "%(", ")")
	return m.tplVars(str, vars)
}

func (m Msg) tplTags(str string, pairs [][]int) string {
	for _, p := range pairs {
		start, end := p[0], p[1]
		text := str[start+2 : end]
		varname, text := zstring.Split2(text, " ")

		key := varname
		if m.oneVar {
			key = ""
		}
		value, ok := m.data[key]
		if !ok { // TODO: update CLI to detect this
			str = str[:start] + "%(z18n ERROR: no value for " + varname + ")" + str[end+1:]
			continue
		}
		t, ok := value.(Tagger)
		if !ok { // TODO: update CLI to detect this.
			str = str[:start] + "%(z18n ERROR: value for " + varname + " is not a Tagger)" + str[end+1:]
			continue
		}

		str = str[:start] + t.Open() + text + t.Close() + str[end+1:]
	}
	return str
}

func (m Msg) tplVars(str string, pairs [][]int) string {
	for _, p := range pairs {
		start, end := p[0], p[1]
		varname := str[start+2 : end]

		key := varname
		if m.oneVar {
			key = ""
		}
		val, ok := m.data[key]
		if !ok { // TODO: update CLI to detect this
			str = str[:start] + "%(z18n ERROR: no value for " + varname + ")" + str[end+1:]
			continue
		}

		str = str[:start] + zstring.String(val) + str[end+1:]
	}
	return str
}

// String displays this string as "other".
func (m Msg) String() string {
	// TODO: implement plurals.

	if m.Other != "" {
		return m.tpl(m.Other)
	}
	return m.ID
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

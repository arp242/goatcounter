z18n is a i18n library for Go.

Import as `zgo.at/z18n`.

The chief motivation for writing this is that I wanted a nice painless (as
painless as i18n gets anyway) API, and none of the existing solutions that I
could find really offered this, not without some extensive wrapping anyway.

[github.com/nicksnyder/go-i18n][go-i18n] was an important inspiration, and z18n
includes some code from it (why reinvent stuff you can copy right?) Standing on
the shoulders of giants and all that. This was written for [GoatCounter][gc],
and was (indirectly) sponsored by [NLnet NGI0][ngi].

It supports pluralisation (copied from go-i18n), named variables using an easier
syntax than text/template, placeholder syntax for HTML tags, localisation of
dates and number, various different message formats (Go source files, Gettext
.po files, SQL, TOML, YAML, JSON), and includes a web interface to translate
messages.

README index:

- [Adding it to an application]()
- [Translating messages]()
  - [Variables]()
  - [HTML]()
  - [Plurals]()
  - [Localisation of dates, numbers]()
  - [Adding context]()
- [Using from templates]()
- [JavaScript]()
- [Finding messages and creating translation files]()
- [Accepting translations and updates from translators]()

[go-i18n]: https://github.com/nicksnyder/go-i18n
[gc]: https://www.goatcounter.com
[ngi]: https://nlnet.nl/PET/

Adding it to an application
---------------------------
Create a new bundle, in this case with the default language set to British
English:

	b := NewBundle(language.MustParse("en-GB"))

The "default language" is the "native" language of the application; I will use
`en-GB` here, but there is nothing stopping you from writing an application in
Russian and translating that to English.

A "bundle" is a set of all translations your application has. You can add
translated messages to it:

	b.AddMessages(language.MustParse("en-GB"), map[string]Msg{
		"insult/cow":   Msg{Default: "You fight like a dairy Farmer!"},
		"comeback/cow": Msg{Default: "How appropriate. You fight like a cow!"},
	})

	b.AddMessages(language.MustParse("nl-NL"), map[string]Msg{
		"insult/cow":   Msg{Default: "Je vecht als een melkboer!"},
		"comeback/cow": Msg{Default: "Erg toepasselijk. Je vecht als een koe!"},
	})

You can also use `language.English` and `language.Dutch`; but I prefer to add
the region tags. It makes it more explicit and easier to add regional variations
later on.

Get a locale from the bundle:

	l := b.Locale("nl-NL")

Which can be used to display a translated message:

    l.T("insult/cow")

More details on how to translate messages in the section below, but just to give
a quick taste of what it looks like:

    // Embed the Default message:
    l.T("comeback/rubber|I am rubber, you are glue!")

    // Placeholder variables:
    l.T("comeback/rubber|I am rubber, you are %(you)", "glue")
    l.T("comeback/rubber|I am %(i), you are %(you)", z18n.P{
        "i":   "rubber",
        "you": "glue",
    })

    // HTML tags:
    l.T("comeback/rubber|I am %[link-i rubber], you are %[link-you %(you)]", i18n.P{
        "link-i":   z18n.Tag("a", `href="https://en.wikipedia.org/wiki/Natural_rubber"`),
        "link-you": z18n.Tag("a", `href="https://en.wikipedia.org/wiki/Adhesive"`),
        "you":      "rubber",
    })

A bundle only needs to be created once; if you're using this in a long-running
webapp then create a bundle on startup and a locale for every user/request based
on the user settings, `Accept-Language` header, etc.

There is also a top-level `z18n.T` which takes the Locale object from a context,
which can be created with `z18n.With()`:

    ctx := z18n.With(context.Background(), l)
    z18n.T(ctx, "insult/cow")

Translating messages
--------------------
The `T` function accepts the message ID as the first parameter:

    l.T("song/coconuts")

This will look up the message with the ID `song/coconuts`. A message ID can
contain any character except a `|`.

You can optionally specify a default message after a `|`; this makes grepping
code and such a bit easier, and means you won't have to translate every single
last string in the z18n files:

    l.T("song/coconuts|I've got a lovely bunch of coconuts!")

You can only set the default (unpluralized) message with this.

The `song/` doesn't mean anything special, it's just a convention that might be
useful. You can also use `song-coconuts`, `song#coconuts`,
`song/silly/coconuts`, or just not use any prefix at all and use only
`coconuts`. Personally I found using prefixes useful so I'll use them in this
documentations

### Variables
Variable interpolation works with `%(varname)`; the `varname` should remain
identical in translated messages as it's used to lookup the variable:

    l.T("email-me|Email me at %(email)", email)

Variable names can contain any character except whitespace and `)`. If you have
only one variable then you can pass it as just an argument, but if if you have
more than one you will need to use a map:

    l.T("email-me|Email me at %(email) or %(email2)", z18n.P{
        "email": email,
        "email": email2,
    })

The reason for this is that the location of the variables might be different in
translated messages.

Variables can have functions:

    %(word)                 Just a variable
    %(word upper_first)         Apply function
    %(word lower upper_first)   Apply two functions

Which is useful sometimes to deal with capitalisation in some languages.
Supported functions:

    lower           Lower-case everything.
    upper           Upper-case everything.
    upper_first     Upper-case the first letter, leaving the rest of the case intact.
    raw             Don't locale numbers or dates.
    html            Escape as HTML string, for when Bundle.NoHTML is set.

Variable values are always HTML-escaped by default unless `Bundle.NoHTML` is
set.

### HTML
You can use `%[varname text]` as a placeholder for HTML tags; this is intended
to be used with the `z18n.Tag()` function and removes the need for (most) HTML
inside translation strings, and makes updating links etc. easier:

    l.T("docs|You can find out more %[from the documentation]")
        z18n.Tag("a", `href="/docs" class="link"`))

The first parameter (`a`) is the tag name, and the second whatever you want to
put in the opening tag.

As with `%(..)` variables you can pass it as just an argument if you have just
one value, but will need to use a map if you use multiple variables and/or tags.
Variables can be used inside `%[..]` tags:

    l.T("email-me|%[link Email me at %(email)] to find out more", z18n.P{
        "link":  z18n.Tag("a", `href="mailto:me@example.com"`),
        "email": email,
    })

It's not possible to nest these tags: `%[one %[two tags]]` won't work. You can
create your own type which implements the `z18n.Tagger` interface if you need
some more complex HTML.

You can use `TagNone()` to not add any tags; this is useful in some contexts
just to signal to the translator that this will be an actionable element:

    l.T("send|%[Send]", z18n.TagNone())

This would result in just `Send`.

### Plurals
Thus far we've only set `Msg.Default`; this is the message to use if there are
no pluralisations to apply; there are five other messages:

    One, Zero, Two, Few, Many

z18n will use one of these (or the `Default`) automatically when supplied with a
`Plural` value. Leaving the appropiate value empty will result in an error.

The logic for all of this can actually be quite complex and often includes
exceptions as well – as languages do. Plurals in English (and most Germanic
languages) are usually fairly easy with just "one" and "more than one". Many
Asian languages like Indonesian have it even easier by just not having plural
forms at all, and Polish people [must have a Ph.D. in math embedded in their
DNA][pl].

Anyway, to add Plurals to the messages use the appropriate field(s):

	b.AddMessages(language.BritishEnglish, map[string]Msg{
        "ants!": Msg{
            One:   "Help, I've got an ant in my trousers!"
            Default: "Help, I've got %(n) ants in my trousers!"
        },
    })

	b.AddMessages(language.AmericanEnglish, map[string]Msg{
        "ants!": Msg{
            One:     "Help, I've got an ant in my pants!",
            Default: "Help, I've got %(n) ants in my pants!",
        },
    })

	b.AddMessages(language.Indonesian, map[string]Msg{
        "ants!": Msg{
            One:     "Tolong, saya punya semut di celana saya! ",
            Default: "Tolong, saya punya %(n) semut di celana saya!",
        },
    })

	b.AddMessages(language.Polish, map[string]Msg{
        "ants!": Msg{
            One:  "Pomocy, mam mrówkę w spodniach!",
            Two:  "Pomocy, mam %(n) mrówki w spodniach! ",:
            Few:  "Pomocy, mam %(n) mrówek w spodniach! ",:
            Many: "Pomocy, mam w spodniach %(n) mrówek!",:
        },
    })

To tell z18n which form to use, pass a `z18n.Plural`; the `z18n.N()`
conveniently creates this without too many characters:

    l.T("ants!", z18n.N(5))

This can be in any position and will automatically be made available as the
variable `n`, and can of course be combined with other variables:

	b.AddMessages(language.BritishEnglish, map[string]Msg{
        "marketers": Msg{
            One:   "I emailed %(email) only once with my stupid marketing offer, so better send 5 more"
            Default: "I emailed %(email) %(n) times with my stupid marketing offers",
        },
    })

    l.T("marketers", z18n.N(51), email)

[pl]: plural/rule_gen.go#344

### Localisation of dates, numbers
Numbers and `time.Date` will be formatted according to the locale, for example:

    l.T("id|number: %(n); float: %(f); time: %(t)", z18n.P{
        "n": 1_230_495,
        "f": 6666.42,
        "t": time.Now(),
    })

This will show:

    en-US       1,230,495   6/18/21         American "reverse" format.
    en-NZ       1,230,495   18-06-2021      Standard format for New Zealand.
    nl-NL       1.230.495   18-06-2021      Appropiate digit groupings for the language.

This is one reason that `language.MustParse(`en-NZ`) is better than
`language.English`.

The formats can be overridden for a locale, in case the user set something
different. You usually want to provide some feature for this: there are
probabably a number of people who use English, but aren't used to the US date
style, and some people prefer may `2006-01-02` (or maybe some other format) as
well.

    l := bundle.Locale(language.MustParse("en-US"))

    // Set dates; datetime format is derives from this.
    l.Date("2006-01-02")
    l.DateMedium("2 Jan 2006")
    l.DateLong("2 January 2006")
    l.Time("15:16")

    // Thousands and fraction separators.
    l.Thousands('.')
    l.Fraction('.')

You can specify which format to use with a function in the variable:

    Default is to print the datetime:

    %(d)                Jan 02, 2006 2:22 pm        2 Jan 2006 14:22
    %(d short)          02/01/06 2:22 pm            01-02-2006 14:22
    %(d long)           January 2, 2006 2:22 pm     2 January 2006 14:22

    Or print just the date:
    %(d date)           02/01/06                    01-02-2006
    %(d date-short)     January 2, 2006             2 January 2006
    %(d date-long)      January 2, 2006             2 January 2006

    Just the time
    %(d hour)           2:22                        14:22

    Extract specific parts:
    %(d day)            Monday              Maandag
    %(d month)          March               Maart

    Or use a custom format:
    %(d 2006-01-02)     2006-01-02          2006-01-02

TODO: actually we do need ordinals for "2nd of january".

Things like ordinals, formatting of bytes, etc. aren't implemented; use
something like [github.com/dustin/go-humanize][humanize] if you need this.

Use the `raw` function in variables to prevent formatting:

    l.T("id|number: %(n raw); float: %(f raw); time: %(t raw)", z18n.P{
        "n": 1_230_495,
        "f": 6666.42,
        "t": time.Now(),
    })

[humanize]: https://github.com/dustin/go-humanize

### Adding context
There are two ways to add context; the first is in the message ID; for example:

    l.T("button/get-quote|Get")

This makes it clear that "get" is used as a form button to get a quote. This may
be important, because words like "get" can sometimes be translated in multiple
ways, and not all of them may be appropriate in this context. This is one reason
I like using prefixes, because now it's pretty clear that this is a button that
*does* something (but you can also use `get-quote-button`, if you prefer).

A second way is to use special comments:

    // z18n: Context
    // Context continues.
    l.T("button/get-quote|Get")

Or:

    /* z18n: Context
       Context continues. */
    l.T("button/get-quote|Get")

Or:

    l.T("button/get-quote|Get") // z18n: some context
    l.T("button/get-quote|Get") /* z18n: some context */

I generally I would recommend avoiding this unless necessary; good ids are
better.

The comments *need* to be prefixed with `z18n:` and immediately precede the `T`
call. This **won't** work:

    // z18n: there is a blank like.

    l.T("button/get-quote|Get")

    // z18n There is no ":" after z18n.
    l.T("button/get-quote|Get")


Using from templates
--------------------
You can add `z18n.Thtml` to the template function list:

	tplfunc := template.TplFunc{
        "t":   z18n.Thtml,
        "tag": z18n.Tag,
    }

And then use:

    {{t .Context "id|My message!"}}

Whitespace after at the start and end will be stripped a all other whitespace
will be collapsed to a single space, so multi-line messages work well:

    {{t .Context `id|
        My message!
    `}}

The downside of this is that you need to pass the context every time, You can
create a "scoped" version by assigning a variable:

    b := NewBundle(...)
    l := b.Locale()

    tpl.ExecuteTemplate("foo.gohtml", struct {
        T func(string, ...interface{}) template.HTML
    }{l.T})

And then use it like:

    {{.T "id|My message!"}}

You can add context with `{{/* z18n: ... */}}` with the same rules as Go files.


JavaScript
----------
z18n is a Go i18n tool, not a JavaScript one; there isn't great support for
JavaScript right now.

That said, it's not uncommon to have an application where almost all of the text
is in the backend, with just a few messages in the frontend. The general
strategy would be to render the messages you need server-side and then load them
in JS, this can be:

    <span id="z18n" style="display: none"
        data-msg-one="{{.T "id/msg"}}
        data-msg-two="{{.T "id/msg"}}
    ></span>

Or you can format it as JSON, or load it as JSON from an endpoint. You'll have
to write a simple lookup function yourself, as well as support for variables if
you really need it.

If you need something more complex then you can use one of the many i18n
JavaScript libraries or whatever's bundled in your framework and load the
messages through a JSON endpoint.


Finding messages and creating translation files
-----------------------------------------------
The `./cmd/z18n` tool can find messages in Go and template files.

The simplest usage is:

    $ z18n ./...

This will scan for Go files and templates. See `-help` for various options.

There are a few output types:

    JSON, TOML, YAML    Load from the filesystem (or with Go embed)
    po                  Gettext-compatible; a useful additiona as there are many
                        translation tools for it.
    Go                  Go files, compiled directly in the application.
    SQL                 SQL for loading from a database. The built-in
                        translation tool uses this.

All formats can be freely converted with `z18n convert`, so you can always
change your mind later.

If you use Go files you just need to call the function:

    TODO: language tag should be in the file/struct; we need a different API for
    this, or maybe just store it in msg package.
    b.AddMessages(language.Dutch, msg.NL_NL())

For JSON,, TOML, YAML, and po files call:

    err := b.ReadMessages(language.Dutch, "file.toml")

And for SQL:

    conn := sql.Open(..)
    err := b.FromSQL(conn)


Accepting translations and updated from translators
---------------------------------------------------
You can:

- Send people the "raw" files with the English strings so they can translate it
  (TOML is probably the easiest). Probably works well for more tech-y people.

- Use the built-in web interface.

- Generate .po files and use one of the many frontends for that.

After an update:

- Generate a new base file
- Merge with existing translation file
- Send to translator
- Replace file

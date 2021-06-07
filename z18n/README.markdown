z18n is a i18n library for Go.

Import as `zgo.at/z18n`.

Usage
-----
Create a new bundle, in this case with the default language set to English:

	b := NewBundle(language.English)

And add messages:

	b.AddMessages(language.English, map[string]Msg{
		"hello":     Msg{other: "Hello"},
		"hello-loc": Msg{other: "Hello, %(loc)!"},
	})

	b.AddMessages(language.Dutch, map[string]Msg{
		"hello":     Msg{other: "Hallo"},
		"hello-loc": Msg{other: "Hallo, %(loc)!"},
	})

Get a locale:

	l := b.Locale("nl_NL")

And finally translate stuff:

    l.T("message-id")

A bundle only needs to be created once; if you're running this in a long-running
web-app then create a bundle on startup, and a locale for every user/request
based on the user settings, `Accept-Language`, etc.

There is also a top-level `z18n.T` which takes the Locale object from a context,
which can be created with `z18n.With()`:

    ctx := z18n.With(context.Background(), l)
    z18n.T(ctx, "message-id"")


Translating messages
--------------------
The `T` function accepts the message ID as the first parameter, everything else
are placeholders for interpolation; for example:

    l.T("prefix/coconuts")

Will look up the message with the ID `prefix/coconuts`. A message ID can contain
any characters except a `|`.

You can (optionally) specify a default message afer a `|`; this makes grepping
code and such a bit easier, and means you won't have to translate every single
last string in the i18n system:

    l.T("prefix/coconuts|I've got a lovely bunch of coconuts!")

The `prefix/` doesn't mean anything, it's just a convention that might be
useful. You can also use `prefix-coconuts`, `prefix#coconuts`,
`prefix1/prefix2/coconuts`, or just not use any prefix at all and use only
`coconuts`.

### Variables

Variable interpolation works with `%(varname)`; the `varname` should remain
identical in translated messages, as it's used to lookup the variable.

    l.T("email-me|Email me at: %(email)", email)

The placeholder syntax is as follows:

    %(word)                 Replace
    %%(word)                Literal "%(word)"
    %(word ucfirst)         Apply function
    %(word lower ucfirst)   Apply two functions

If you have only one variable then you can pass it directly; if you have more
than one you will need to use a map:

    l.T("email-me|Email me at: %(email) or %(email2)", z18n.P{
        "email": email,
        "email": email2,
    })

### HTML

You can use `%[varname text]` as a placeholder for HTML tags; this is intended
to be used with the `z18n.Tag()` function. This removes the need to put (most)
HTML inside translation strings, and makes updating links etc. easier:

    l.T("docs|You can find out more %[from the documentation]")
        z18n.Tag("a", `href="/docs" class="link"`))

As with %(..) variables, you can pass it directly for a single parameter but
will need to use a map if you use multiple.

Variables can be used inside `%[..]` tags:

    l.T("email-me|%[link Email me at %(email)] to find out more", z18n.P{
        "link": z18n.Tag("a", `href="mailto:me@example.com"`),
        "email": email,
    })

It's not possible to nest these tags: `%[%[two tags]]` won't work. You can add
your own tags by implementing the `Tagger` interface, for example if you need
more complex HTML (i.e. a `<form>` for POST buttons).

You can use `TagNone()` to not add any tags; this is useful in some contexts
just to signal to the translator that this will be an actionable element:

    l.T("send|%[Send]", z18n.TagNone())

### Adding context

There are two ways to add context; the first is in the message ID; for example:

    l.T("button/get-quote|Get")

This makes it clear that "get" is used as a form button to get a quote. This may
be important, because words like "get" can sometimes be translated in multiple
ways, and not all of them may be appropriate in this context.

A second way is to use special comments:


    // z18n: Context
    // Context continues.
    l.T("button/get-quote|Get")

Or:

    /* z18n: Context
       Context continues. */
    l.T("button/get-quote|Get")

Or:

    l.T("button/get-quote|Get") // z18n: some context.

But generally I would recommend avoiding this unless neccisary. Good ids are
better.

Using from templates
--------------------

You can add `z18n.T` to the template function list:

	tplfunc := template.TplFunc{
        "t": func(ctx context.Context, msg string, data ...interface{}) template.HTML {
            return z18n.T(ctx, msg, data...)
        },
    }

And then use:

    {{t .Context "id|My message!"}}

Or multi-line:

    {{t .Context `id|
        My message!
    `}}

The downside of this is that you need to pass the context every time, You can
create a "scoped" version by assigning a variable:

    tpl.ExecuteTemplate("foo.gohtml", map[string]interface{}{
        "T": func(msg string, data ...interface{}) {
            return template.HTML(z18n.T(g.Context, msg, data...))
        },
    })

Or:

    func (g Globals) T(msg string, data ...interface{}) template.HTML {
        return template.HTML(z18n.T(g.Context, msg, data...))
    }

And then use it like:

    {{.T "id|My message!"}}

You can add context with `{/* z18n: ... */}`.


JavaScript
----------
TODO: figure this out; at least make it easy to provide a struct/JSON of
(select) messages.


Finding messages and creating translation files
-----------------------------------------------
The `./cmd/zli` tool can find messages in Go and template files.

TODO: document further when ready.


[go-i18n]: https://github.com/nicksnyder/go-i18n

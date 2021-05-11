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

There is also a top-level `z18n.T` which takes the Locale object from a context:

    ctx := z18n.With(context.Background(), l)
    z18n.T(ctx, "message-id"")

Translating messages
--------------------
The `T` function accepts the message ID as the first parameter, everything else
are placeholders for interpolation.


    l.T("prefix/asd")

A message ID can contain any characters except a `|` and it can't start with a
`!`.

You can (optionally) specify a default message right in the string; this makes
grepping code and such a bit easier, and means you won't have to translate every
single last string in the i18n system.

Specify the default message after a `|`; any further `|`s are ignored and
treated as just part of the message:

    l.T("prefix/asd|default msg")

The `prefix/` doesn't mean anything, it's just a convention that might be
useful. You can also use `prefix-asd`, `prefix#asd`, `prefix1/prefix2/asd`, or
just not use any prefix at all.

Variable interpolation works with `%(word)`; the "word" doesn't really mean
anything, they're just positional as with printf, but it can help clarify the
context for translators:

    l.T("prefix/asd", email)
    l.T("prefix/asd|default msg %(email)", email)

Placeholder syntax is as follows:

    %(word)                 Replace
    %%(word)                Literal "%(word)"
    %(word ucfirst)         Apply function
    %(word lower ucfirst)   Apply two functions

You can use `[[word]]` to signify that a piece of text is a button; this is
purely decorational and to make strings easier to understand for translators:

    l.T("btn|[[Resend]]")


[go-i18n]: https://github.com/nicksnyder/go-i18n

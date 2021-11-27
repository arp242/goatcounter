{{template "%%top.gohtml" .}}

Translating GoatCounter
=======================
To translate GoatCounter copy `i18n/base.toml` to `i18n/<language>.toml` and:

1. Change `base-file = false` to `base-file = true`.
2. Set the `language`.
3. Translate the content :-)

You can open a pull request with the new file, or send it by email. After
placing the file there with the correct `language` set it should show up in the
settings after a restart.

Translations are done with [z18n].

[z18n]: https://github.com/arp242/z18n

Syntax
------
Translation strings support variables, which look like:

    Hello, %(name)

The `%(name)` is a variable that will be injected in the string. It can be
anywhere in the string but you *must* keep the same variable name.

---

A second type of variable looks like:

    Click %[here]

The `%[..]` signifies that will be wrapped in some HTML, such as a link or
button, or just bold text. The `here` will be put inside the HTML and should be
translated.

A related form is:

    Click %[%varname here] or %[%othervar there].

Here, `%varname` and `%othervar` are the names of the variables, and should
*not* be translated. The `here` and `there`` should though.

You can put variables inside here too:

    Feel free to %[%link email me at %(email)] if you have any questions.

---

You don't need to worry about correct localised formatting of dates, numbers,
currency, etc. This is handled automatically.

---

All translation IDs are in the form of `<prefix>/<id>`; the `prefix` gives a bit
of context where a translation is used:

    header/    Headers, h1, h2, etc.
    label/     Form labels
    button/    Form buttons
    help/      Form help text
    notify/    Flash messages
    error/     Error messages
    link/      Links
    p/         Paragraphs of text
    top-nav/   Navigation at top
    bot-nav/   Navigation at bottom
    dash-nav/  Dashboard navigation

Updating translations
---------------------
TODO...


{{template "%%bottom.gohtml" .}}

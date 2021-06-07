A translation string looks like:

    Hello, %(name)

The `%(name)` is a variable that will be injected in the string. It can be
anywhere in the string but you *must* keep the same variable name.

You can use functions inside `%(name)` as well:

    Hello, %(name ucfirst)

The functions may differ per translation; this can be useful to adhere to things
like capitalisation rules and such.

Functions:

    lower         lower-case the entire string.
    upper         upper-case the entire string.
    upper_first   upper-case the first letter of the string.

You don't need to worry about correct localised formatting of dates, numbers,
currency, etc. This is handled automatically.


---

A second type of variable looks like:

    Click %[name here]

The `%[..]` signifies that will be wrapped in some HTML, such as a link or
button, but it may also be

The `name` is the variable name, and must be kept intact. Everything after this
will be translated.

You can put variables inside here too:

    Feel free to %[email email me at %(email)] if you have any questions.

---

All translation IDs are in the form of `<prefix>/<id>`; there are a few standard
prefixes:

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

----

Files to be translated:

    handlers/backend.go
    handlers/billing.go
    handlers/dashboard.go
    handlers/settings.go
    handlers/settings_user.go
    handlers/user.go

    tpl/_dashboard_pages.gohtml
    tpl/_dashboard_pages_text.gohtml
    tpl/_dashboard_totals.gohtml
    tpl/settings_changecode.gohtml
    tpl/settings_delete.gohtml
    tpl/settings_export.gohtml
    tpl/settings_main.gohtml
    tpl/settings_sites.gohtml
    tpl/settings_sites_rm_confirm.gohtml
    tpl/settings_users.gohtml
    tpl/settings_users_form.gohtml
    tpl/user_api.gohtml
    tpl/user_pref.gohtml

    tpl/_email_bottom.gotxt
    tpl/email_adduser.gotxt
    tpl/email_export_done.gotxt
    tpl/email_forgot_site.gotxt
    tpl/email_import_done.gotxt
    tpl/email_import_error.gotxt
    tpl/email_password_reset.gotxt
    tpl/email_verify.gotxt
    tpl/email_welcome.gotxt

    public/backend.js

The design and implementation of GoatCounter
--------------------------------------------

Today I released [GoatCounter](https://www.goatcounter.com). It's my take on
"web analytics", although it's not really "analytics" since it merely "counts"
page hits, rather than perform extended analytics based on user tracking.

Why "GoatCounter"? It's written in Go, I did `grep ^[gG]o /usr/share/dict/*`,
and "goat" is what I came up with. Also, zgo.at was (surprisingly!) still
available and it's a cute domain hack.

Some of the design choices made for GoatCounter will probably be considered
idiosyncratic. They were, however, very deliberate.

The backend is written in Go. The frontend is basic JavaScript with jQuery (yes,
jQuery). The primary database engine is SQLite, which will work fine for most
people, but PostgreSQL is also supported (chiefly for the hosted service I'm
offering).


Rationale and user experience
-----------------------------

Before I go in to some technical details I should explain what I hope to
achieve.

I only want to answer a single question: "which pages are being viewed on my
site, and where are people coming from?"

This is often the only question people really want to extract from analytics
tools. Software such as Google Analytics or the open source Matomo (formerly
Piwik) are great, but far too complex for this goal. I tried Matomo for a few
weeks on my site and aside from adding far too much JavaScript, I could never
figure out to get a clear answer to that question.

I didn't like most other existing solutions for various reasons, and the only
one with potential seems Fathom, but it doesn't seem very active (most PRs and
issues seem ignored or unresolved), is somewhat hard to self-host, and wanted to
make some different design decisions.

First and foremost I want to give users a very good *Just Works™* user
experience (UX). Long waiting times, laptop fans that start spinning, inability
to open links in a new tab, etc. are *NOT* good UX.

Drew DeVault's SourceHut was a major inspiration.


Backend
-------

The backend is written in Go. Why Go? It's reasonably fast, easy to read and
write, and the entire program can be compiled to a single standalone binary.

### Configuration

All configuration is done via commandline flags. I wrote [Flags are great for
configuration](/flags-config.html) and [Using flags for configuration in
Go](/flags-config-go.html) about that before, so I won't expand on that too much
here.

See: `cfg` package.

### zhttp

Everyone needs their own HTTP framework, right?

https://github.com/zgoat/zhttp

zhttp is not necessarily intended to be a "general use" HTTP framework. In fact,
it's hardly a framework at all, and more a set of functions.

There are many assumptions. "Convention over configuration" and all that. I plan
to write a few more webapps with similar design aesthetics, so this will be
useful for that (in fact, I originally wrote it for new "GoatLetter" newsletter
service, which I should finish in a month or so).


### Pack

The entire program compiles to a single binary, including all templates and
static assets. The only external dependency is a SQLite database file or
PostgreSQL connection.

Running `go get zgo.at/goatcounter && ~/go/goatcounter` is all you need to get
running (the SQLite database file will be created). I think this is a good
feature, as even less tech-savvy people will be able to run GoatCounter.

While there are many existing programs/libraries to achieve this, I decided for
a simple "KISS" approach: https://github.com/zgoat/zhttp/blob/master/pack.go

The logic is essentially:

    if production {
        data = packed[filename]
    } else {
        data = fromFilesystem[filename]
    }

It's pretty simple, doesn't require any tools, and a standard `go generate
./...` is enough.

### zlog

[zlog](https://github.com/zgoat/zlog) is my logging library. It's simple and
effective.

The design is based on an internal logging library used at Teamwork (my previous
employer), which is a wrapper around Logrus. Various design decisions make it
hard to generalize (which is why I never open sourced it), and this is my
attempt at "Teamwork's log library done right".


- User management
- Cron
- Pregenerated stats
- Templates
- go generate
- config: arp242.net/go-config.html


Frontend
--------

Just plain ol' template-driven app with jQuery. I know, I know; this is not how
you're supposed to do things now, but to hell with that.

Charts
------

The charts are generated on the server. This may seem like an odd choice, since
it's comparatively expensive (hence the popularity of JS-based solutions), but
that doesn't make it cheaper: it merely makes your laptop's fans spin, instead
of my server's.

I think making my server's fans spin faster is the more user-friendly option. It
also enables some smarter caching: we can store the generated HTML for past
dates and merely retrieve this, instead of generating new HTML every time (this
is not yet implemented).


Sysops
------
Deploy is done using a simple shell script. After deploy I restart manually.

Not a true "CI pipeline", but works well enough.

This approach is simple and easy to set up, with little overhead and complexity.
It doesn't scale to a team of 10 people, but it's not intended to.
Not everything needs to scale.
It's unlikely that there will ever be a team working on GoatCounter, and if
there is it'll be easy to rethink (parts of) the approach. It will be a
significantly smaller fraction of the total workforce.

- Varnish
- Caddy

The design and implementation of GoatCounter
--------------------------------------------

Backend
-------
The backend is written in Go. Why Go? The first reason is one of practicality:
it's what I've done most of my programming in for the last three years (as my
job), so it's "in my fingers", so to speak.

There are also some good advantages: it's pretty fast and I can compile a single
standalone binary.

- zlog
- zhttp
- User management
- Cron
- Pregenerated stats
- Templates


Frontend
--------
Just plain ol' template-driven app with jQuery. I know, I know; this is not how
you're supposed to do things now, but to hell with that.


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

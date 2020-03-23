Session tracking
================

"Session tracking" allows more advanced tracking than just the "pageview"
counter we have now. A "session" is a single browsing session people have on a
website.

Right now, ever pageview shows up as-is in the dashboard, including things like
page refreshes. There is also no ability to determine things like conversion
rates and the like.

Goals:

- Avoid requiring GDPR consent notices.

- The ability to view the number of "unique visitors" rather than just
  "pageviews".

- Basic "bounce rate" and "conversion rate"-like statistics; for example, if
  someone enters on /foo we want to be able to see how many leave after visiting
  just that page, or how many end up on /signup.

Non-goals:

- Track beyond a single browsing session.


Existing solutions
------------------

An overview of existing solutions that I'm aware or with roughly the same goals.

Ackee
-----

https://github.com/electerious/Ackee/blob/master/docs/Anonymization.md

> Uses a one-way salted hash of the IP, User-Agent, and sites.ID. The hash changes
> daily and is never stored.
>
> This way a visitor can be tracked for one day at the most.

This seems like a decent enough approach, and it doesn't require storing any
information in the browser with e.g. a cookie.

It does generate a persistent device-unique identifier though, and I'm not sure
this is enough anonymisation in the context of the GDPR (although it may be?
It's hard to say anything conclusive about this at the moment)

Fathom
------

https://usefathom.com/blog/anonymization

> Unique siteviews are tracked by a hash which includes the site.ID; unique
> pageviews are tracked by as hash which includes the site.ID and path being
> tracked.
>
> To mark previous requests "finished" (not sure what that means) the current
> pageview's hash is removed and moved to the newest pageview.

I'm not entirely sure if it's actually better or more "private" than Ackee's
simpler method. The Fathom docs mention that they "can’t put together an
anonymous, individual user’s browsing habits", but is seeing which path people
take on your website really tracking someone's "browsing habits", or can this
lead to identifying a "natural person"?

Or, to give an analogy: I'm not sure if there's anything wrong with just seeing
where your customers go in your store. The problems start when you start
creating profiles of those people on recurring visits, or when you see where
they go to other stores, too.


SimpleAnalytics
---------------

https://docs.simpleanalytics.com/uniques

> If the Referer header is another.site or missing it's counted as a unique
> visit; if it's mysite.com then it's counted as a recurring visit.

A lot of browsers/people don't send a Referer header (somewhere between ~30% and
~50%); this number is probably higher since Referer is set more often for
requests in the same domain, but probably not 100%.

This is a pretty simple method, but it doesn't allow showing bounce or
conversion rates or other slightly more advanced statistics.


GoatCounter's solution
----------------------

- Create a server-side hash: hash(site.ID, User-Agent, IP) to identify the
  client without storing any personal information directly, and the site.ID
  ensures users aren't identifiable across sites.

- An hour after a hash is last seen, the hash is replaced with a random number.
  This ensures we can track the current browsing session only, without some
  arbitrary cut-off at midnight.

- If a user visits the next day, they will have the same hash, but the system
  has forgotten about it by then.

### Technically

We can store the data in a new `session` table and link that to every hit:

	create table sessions (
		site         int,
		hash         varchar,
		created_at   datetime,
		last_seen    datetime
	);
	alter table hits add column session int default null;

To efficiently query this a new `stats_unique` or `count_unique` column can be
added to all the `*_stats` tables, which is a copy of the existing columns but
counting only unique requests.

In the UI we can add a new "show unique visitors" button next to "view by day",
or perhaps change the UI a bit to show both.

We can add bounce rates to every path, as well as a dashboard thingy for "top
bounce rates" or the like.

Not entirely sure what I want to conversion rates UI to look like. This also
requires a new settings tab etc. and is a separate issue.

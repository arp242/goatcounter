This document describes long-term goals for GoatCounter.

Right now, GoatCounter just "counts" who visits what page. This is a good start
but I'd like to add a few more advanced features to give better insight. The
end-goal is to give meaningful analytics (instead of just "vanity analytics")
while still maintaining reasonable and expected levels of privacy (i.e. it won't
be possible to connect data to a real person, or to see everything someone did
on your website ever).

A high-level overview, in order of what needs to be done:

- There are various "plumbing" tasks that need to be done, like supporting
  multiple users, making it easier to self-host an instance (https server,
  ACME implementation which doesn't depend on shell scripts, better DB
  migrations/management), and a few performance issues.

- In my own usage I notice I miss various basic features, such as searching
  paths, or the ability to add tracking for "who is clicking on my
  {Patreon,Sign up} button?", or the ability to view dates in the local
  timezone (it's all UTC now for simplicity).

- Setting up a good and highly reliable hosted option; I feel this is
  important because in reality, most people are just not going to self-host
  GoatCounter. Even people who have the required technical skills often have
  better stuff to do than maintain their own. This is an important reason
  for the popularity of Google Analytics: it's free and easy to set up.

  Right now everything runs on a simple $20/month Linode VPS and free Open
  Source Netlify account for CDN. It works well enough for now, but needs
  some more work to ensure better reliability; there have been two problems
  already due to simple config errors, both of which would have been caught
  and fixed very quickly with better monitoring/testing of the live site in
  place. Backups are also a bit ad-hoc right now by downloading them to my
  laptop every day. In other words, it all needs some work.

  The "self-hosted" option will always remain a first-class citizen.

- Better/more flexible charts; right now it's essentially just one view (per
  hour), and it would be helpful to have a per-day view too, or to easily
  compare the current month to the previous.

  I feel this is an essential part of what's missing from a lot of existing
  "simple" solution, which just give you "you had 5k visitors this month".
  That's all great, but it doesn't really tell you stuff like "you had a 9%
  growth this month", or "you had a 3% growth from source A, and a 18%
  growth from source B".

- Give more flexible insight outside of "number of visits per page". A lot
  of use cases revolve around sales rather than page views, and supporting
  complex workflows/insights is what make tools like Matomo and GA so
  complex. The trick is here is to strike a good balance between being easy
  to use/add technically, giving meaningful data, and still being easy to
  use for the average person.

- Make a slightly more advanced "tracker" which allows giving a bit more
  insight in the path visitors take (e.g. "30% of people never complete the
  signup form", or "25% of people who enter via /foo.html go to
  "/bar.html").

  This gives more insight in what kind of pages are effective at driving
  conversion, and also enabling insight in how a change/redesign affects the
  effectiveness of a site.

  The trick here is doing it without pervasive tracking. Fathom does this by
  setting a temporary 30-minute cookie, for example, although I'd like to
  explore alternatives which doesn't store *any* user ID.

Out of scope are things like highly advanced data analysis, user
identifiable tracking, "real time visitor information", or generally
covering every single use case.

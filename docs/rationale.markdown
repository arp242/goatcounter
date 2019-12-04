This document describes the rationale for developing GoatCounter, its goals, and
a comparison with existing solutions.

tl;dr ("elevator pitch"): give meaningful privacy-friendly analytics for
business purposes, while still staying usable for non-technical users to use on
personal websites. It should be free or low-cost for personal use.


Background
----------

I was working on a product idea and wanted to add some basic analytics to
measure how many people are visiting the site, and I've also been wanting to add
basic analytics to my personal homepage/programming weblog to measure if anyone
is reading anything I write.

Even fairly simple analytics are useful to measure things like "what type of
content is popular, and should I write more of?", "does it even make sense to
distribute a newsletter?", or "how does the redesigned signup button affect
signup rates?"

I tried a number of existing solutions, and found existing solutions are either
very complex and designed for advanced users, or far too simplistic. In addition
almost all hosted solutions are priced for business users (≥$10/month), making
it too expensive for personal/hobby use.

What seems to be lacking is a "middle ground" that offers useful statistics to
answer business questions, without becoming a specialized marketing tool
requiring in-depth training to use effectively.

Furthermore, some tools have privacy issues (especially Google Analytics).

Goals
-----

- Give useful data, while respecting people's privacy. For the most part, it
  should just "count events" (hence the name).

  There should always be an option to add GoatCounter to your site *without*
  requiring a GDPR notice.

- Easy UX; some existing solutions are surprisingly complex, to the point where
  I wasn't able to figure some basic stuff out.

- Make a web app that I like using (rather than tolerate it). This is a bit
  subjective, and perhaps my tastes are "old-fashioned", but I find a lot of
  "modern" UIs annoying. Google Analytics is a good example where pressing the
  "back button" will often break everything.

- Works well with any browser and assistive technology, whenever possible.

- Free or very cheap hosted version. Almost all hosted solutions are exclusively
  oriented towards business use. This makes sense from a business point of view
  (better to support 100 customers paying $30 each than 1000 paying $3 each),
  but it does leave a lot of people without a good/affordable solution.

- Easy to run your own. `go install zgo.at/goatcounter` should be all you need
  to get started. No need to set up web servers, PHP, MySQL databases,
  what-have-you (binaries should be provided, too).

  I feel this is an important feature, because "run your own" sounds nice, but
  it becomes a bit of a niche feature if you need to have a lot of knowledge and
  spend a lot of time setting everything up. Ain't got no time for that.


Existing solutions
------------------

### Matomo (formerly Piwik)

- I found the UX exceedingly hard to use; stuff like "who is visiting which page
  from where?" are *probably* answerable with Matomo, but wasn't able to figure
  out how exactly.

- Large tracking script (192K, or 57K gzip'd).

- No cheap hosted option (cheapest: $19/month).

- Not "true" open source but "open core", although most premium features aren't
  really required for most users.
 
### Fathom

- Open version is no longer maintained in favour of a complete (closed source)
  rewrite, although there is a promise to start maintaining it in the future
  again ([see](https://github.com/usefathom/fathom/issues/268)).

- Hosted is expensive ($14/month).

- Doesn't give referrers per-page (just globally).

- Back button doesn't work.

### Open Web Analytics

- No hosted version.

- Somewhat complex UI with more learning curve than needed. Kind of hard to get
  meaningful data out of it.

- Large-ish payload (73k, 21k gzip'd). While not at large as Matomo, it still
  more than doubles the size of most pages on my website (which are about 25k,
  or 8.5k gzip'd, depending on article length).

- It's a large and IMHO somewhat dated PHP codebase. From a quick inspection, it
  doesn't seem like the kind of codebase I would enjoy working with at all,
  which means that improving it isn't really an option (unlike e.g. Fathom,
  which has a much more workable codebase).

- It's still maintained, but seems to be considered a "finished product". Based
  on the commit log from the last few years I don't expect major changes or
  additions.


### Analysing log files

There are many tools for this, and there have been for decades. There's great
options for many cases but all have the same downsides:

- Needs server access.

- Need server logs in specific format (which is not always trivial to set up).

- Hard to get some common data (like screen size).

### Non-open solutions

While I prefer an open source solution, I'm not fundamentally opposed to closed
solutions.

#### Google Analytics

By far the most popular way, not in the least because it's one of the few
solutions that's free or charge.

- Google is large and collects a massive amount of information for almost every
  internet user. Many people are uncomfortable with this.

  It's kind of tricky to determine what role Google Analytics plays *exactly* in
  this. Google has a single privacy policy for all its products and services;
  there is no "Google Analytics Privacy policy", just the general one which is
  rather vague and open-ended on what kind of data it collects *exactly*. This
  policy allows Google to "combine the information we collect among our services
  and across your devices", but it's unclear how this applies to GA.

  There is an option to disable "data sharing" for "Google products & services",
  which allows Google to combine data. This is enabled by default. It's hard to
  determine how this data is combined *exactly*; at least one help page mentions
  that this enables "Google Analytics collects additional information from the
  DoubleClick cookie (web activity) and from Device Advertising IDs", although
  the inline help gives fraud detection as an example.

- Large-ish payload (73k, 28k gzip'd).

- There is no way to add Google Analytics to your website without displaying a
  GDPR notice. The GA TOS specifically states that you need to have a privacy
  policy and tell people you're using GA to collect data (in practice, many
  sites don't do this though).

- The UI is complex (although better than Matomo), and stuff like back button,
  open in new tab, etc. often don't work. It's not terrible, but also not
  optimal.

- It's free up to 10 million hits/month, which should cover almost all personal
  and small-business use.

#### statcounter

- Cheapest plan is $9/month (the "free" one is 

- Lacks per-page referal view. Various UX items seem clunky.

- Tracking script is 32k (10k gzip'd).

#### Simple Analytics.

- Hosted is expensive (cheapest: $19/month).

- Doesn't give very good/useful data.

- Very small tracking script.

#### getinsights.io

- Cheapest plan is $12/month

- Didn't look at in detail; was only recently started (the founder emailed me,
  which is the reason I know about it).

- Very small tracking script.

#### statcounter.com


#### Other

- I didn't look at quantcast.com, mixpanel.com, similarweb.com in-depth. Both
  seem very complex and enterprise-y products that don't really overlap much
  with the intended usage/audience of GoatCounter.

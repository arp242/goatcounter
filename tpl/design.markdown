{{template "%%top.gohtml" .}}

Notes about GoatCounter's design
================================

GoatCounter’s main design is quite different from various other solutions. The
main difference is that the statistics are displayed per path on the dashboard,
rather than as totals.

I originally developed GoatCounter specifically for my own website/blog, where
“site totals” are fairly useless, and “path totals” are much more useful. I
think this applies for many sites; if you have a webshop with different
`/category-1` and `/category-2` paths then being able to quickly see which
categories are popular and being able to see detailed stats for them seems
pretty useful to me.

The downside of this is that the dashboard can appear as “busy”, “intimidating”,
or “overwhelming” at a glance. A simple “630 visitors today” is certainly a lot
clearer. On the other hand, it’s also a lot less useful for many cases.

Is it “more technical”? I don’t know; maybe. Personally I think it’s just a
*more useful* way to display data, and I think (semi-)serious users of all
technical skills should be able to work with it.

GoatCounter isn’t intentionally different – I just built whatever I thought made
sense. That ended up being somewhat different than many other solutions. Looking
at the current landscape I think that GoatCounter being somewhat different is
not a bad thing; There’s not much point in just making a copy of an existing
product right?

---

Some other points about GoatCounter’s design:

Good user experience, which may not necessarily be the same as “looks nice at a glance”
---------------------------------------------------------------------------------------

GoatCounter is very boring. Not for the sake of it, but it’s just a very
“function over form” kind of design. I generally feel that “flashy” things
aren’t necessarily better. In fact, they’re not infrequently just worse UX.

For example, some people have proposed that the “locations” could be replaced
with a map, which looks nicer. I agree, but is it also easier to use? With the
current “boring” list it tells me everything you more or less need to know in
just a few seconds. With a map, this is much harder.

I would consider GoatCounter to be very user-centred. I feel that in the last
decade or so a lot of questions of “design” have shifted too much towards
“graphical design”, rather than “good user interface design”; these are two very
different things. Not that graphic design or making things look nice isn’t
important, but it matters *less* than user interface design.

Of course, different people have different preferences, which is why I wouldn’t
object to adding something like an *optional* feature to display the locations
overview as a map. But at the same time it seems to me that the current “boring”
list is better for *most* users.

Minimize clicks
---------------
For example for example it shows the “day · month (..)” in the top navigation
as text links quite purposefully, as I find having them there within reach of
a single click is easier than using a drop-down or some other more advanced UI
widget. The more advanced widget would probably *look* better, but isn’t
necessarily easier to use.

Useful aggregate statistics rather than not-so-useful detailed statistics
-------------------------------------------------------------------------
Chromium is just displayed as “Chrome”, as are Opera, Edge, and a bunch of other
Chromium-based browsers. Do you *really* care if someone is using Chrome or
Opera? The reason you care about this information is to be able to make informed
decisions about browser and platform support. Since it’s the same same engine
with the same behaviour, it doesn’t really matter.

Similarly, Firefox on iOS is just displayed as Safari.

I tried Matomo for a while before I built GoatCounter and it displayed a lot of
really detailed information about all sorts of stuff. Quite frankly, almost all
of it was just useless, and getting meaningful aggregate data out of it wasn’t
something I was able to do.

{{template "%%bottom.gohtml" .}}

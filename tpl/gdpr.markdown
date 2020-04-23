{{template "_top.gohtml" .}}

GDPR consent notices
====================

The short version is that GoatCounter *probably* doesn’t require a GDPR consent
notice, on the basis that 1) no personally identifiable information is
collected, and 2) it is in the legitimate interest of a site’s owner to see how
many people are visiting their site. A more detailed rationale is described
below.

Identifiable information
------------------------

The [GDPR][gdpr] applies to data which “*could be attributed to a natural person
by the use of additional information*”, and does “*not apply to anonymous
information, namely information which does not relate to an identified or
identifiable natural person or to personal data rendered anonymous in such a
manner that the data subject is not or no longer identifiable*”.

With the collected data it’s extremely hard to identify a natural person, even
by someone with full access to the database (i.e. me).

It’s prohibitively expensive to retrieve the IP address from the hash. The most
unique information being stored right now is the full `User-Agent` header; which
can be fairly unique (depending on your browser, especially some mobile ones
send a lot of personal information like firmware version) especially when
combined with the country, but even this is very limited.

Other information such as the URL or `Referer` do not relate to an identified
person.

It’s true that certain “additional information” from other parties could reveal
more – such as correlating the `User-Agent` and location – but would be hard,
and the retrieved data would be limited (everyone in Indonesia using Firefox is
a rather large pool of people). To determine whether a personal is identifiable
“*account should be taken of all the means reasonably likely to be used*”, and
this doesn’t strike me as reasonably likely.

If a user (i.e. a customer on your site) would contact me for their rights to
have insight in their data and/or have it removed, then I would have no way to
reliably do so, even if they would provide me with most of their computer’s
information. It might be possible if they provide their browsing history, but if
you have full access to all their browsing data then what do you need
GoatCounter’s data for?


Legitimate interest
-------------------

A second point is that consent is not the only legitimate basis for processing
data; there may also be a legitimate interest: “*The legitimate interests of a
controller (..) may provide a legal basis for processing, (..) taking into
consideration the reasonable expectations of data subjects based on their
relationship with the controller.*”

Insight in how many customers are using your product seems to be a “legitimate
interest” to me, as well as a reasonable expectation. A real-world analogy might
be a store keeping track of  how many people enter through which doors and at
which times, perhaps also recording if they arrived by car, bike, or on foot.

The problems start when the store also records your license plate number, or
creates an extensive profile based on your physical attributes and then tries to
combine that with similar data from other stores. This is essentially what
Google Analytics does, but is rather different from GoatCounter.

A similar argument is made for things like logfiles, which often store more or
less the same information.

I am not the first to arrive at this conclusion:
[Fathom](https://usefathom.com/data) did the same.

Conclusion
----------

In conclusion; it should *probably* be safe to add GoatCounter without a GDPR
consent notice; but there are a few things to keep in mind:

1. The GDPR is fairly new, and lacks case law to clarify what *exactly* counts
   as identifiable personal data.
2. EU Regulations such as the GDPR are interpreted and enforced different across
   member states.
3. Other national laws may apply as well.
4. I am not a lawyer 😅 But the similar Fathom interpretation *has* been
   reviewed by one.

Note that nothing is preventing you from adding a consent notice, if you want to
be sure. There is an example for it on the "Site Code" page in your dashboard.

Other than that, it’s advised you consult a lawyer if you want detailed legal
advice specific to your situation.


[gdpr]: https://eur-lex.europa.eu/legal-content/EN/TXT/PDF/?uri=CELEX:32016R0679

{{template "_bottom.gohtml" .}}

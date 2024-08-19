<h3 id="general">General <a href="#general"></a></h3>
<dl>
<dt id="no-pageviews">I don’t see my pageviews? <a href="#no-pageviews">§</a></dt>
<dd>For reasons of efficiency the statistics are updated once every 10
seconds.</dd>

<dt id="no-data">What does <em>(no data)</em> mean in the referrers list? <a href="#no-data">§</a></dt>
<dd>No Referer was sent; this can mean that the user directly accessed the URL
(e.g. from their bookmarks or typing it), that they disabled sending the Referer
header, or that the link they clicked on disabled the Referer header with
<code>rel="noreferer"</code>.</dd>


<dt id="bots">How are bots and crawlers counted? <a href="#bots">§</a></dt>
<dd>They’re not; all bots and crawlers that identify themselves as such are
ignored.

It’s easy for a malicious script to disguise itself as Firefox or Chrome, and
it’s hard to reliably detect this. In practice it’s unlikely that 100% of all
bots are ignored (this is a general problem with analytics, and not specific to
GoatCounter).</dd>

<dt id="dnt">How is the <code>Do-Not-Track</code> header handled? <a href="#dnt">§</a></dt>
<dd>It’s ignored for several reasons: it’s effectively abandoned with a low
adoption rate, mostly intended for persistent cross-site tracking (which
GoatCounter doesn’t do), and I feel there are some fundamental concerns with the
approach. See
<a href="https://www.arp242.net/dnt.html" target="_blank" rel="noopener">Why GoatCounter ignores Do Not Track</a>
for a more in-depth explanation.

You can still implement it yourself by putting this at the start of the
GoatCounter script:
<pre>&lt;script&gt;
window.goatcounter = {
    no_onload: ('doNotTrack' in navigator && navigator.doNotTrack === '1'),
};
&lt;/script&gt;
&lt;script data-goatcounter="[..]"
    async src="//gc.zgo.at/count.js"&gt;&lt;/script&gt;</pre>
</dd>

<dt id="gdpr">What about GDPR consent notices? <a href="#gdpr">§</a></dt>
<dd>You probably don’t need them. The <a href="{{.Base}}/gdpr">the GDPR page</a> goes in to some detail about this.</dd>

<dt id="custom-domain">How do I set up a custom domain? <a href="#custom-domain">§</a></dt>
<dd>
Add a <code>CNAME</code> record pointing to your GoatCounter subdomain:
<pre>stats   IN CNAME    mine.{{.Domain}}.</pre>

Then update the GoatCounter settings with your custom domain. It might take a
few hours for everything to work. <code>mine.{{.Domain}}</code> will continue to
work.

<em>Note that Custom domains will not prevent adblockers from recognizing
GoatCounter; it’s only intended as a “vanity domain”.</em>
</dd>

{{/* Note: naming this id="adblock" will mean uBlock will remove it... */}}
<dt id="blocked">How do adblockers deal with GoatCounter? <a href="#blocked">§</a></dt>
<dd>
Most of them block goatcounter.com; there’s not much that can be done about
that, and there’s also not much I <em>want</em> to do about this. If people
decide they want to block GoatCounter then they’re free to do so.

By my estimate about a third of pageviews are missed due to adblockers; but this
can vary greatly on the type of site and audience.

That said, there are some options:

<ol>
<li>Self-host GoatCounter; when self-hosting GoatCounter nothing is served from
goatcounter.com or associated domains, and adblockers will not block it.</li>
<li>Import pageviews from logfiles; GoatCounter can import pageviews
from web server logfiles; you can send this data to
goatcounter.com. See <a href="{{.Base}}/help/logfile">the documentation for details</a>.</li>
</ol>
</dd>

<dt id="status-code">Is there any way to record HTTP status codes? <a href="#status-code">§</a></dt>
<dd>
Not directly, but if you include the status code in your error page’s
title you can filter by it. Also see
<a href="https://github.com/arp242/goatcounter/issues/3#issuecomment-578202761">issue #3</a>.
</dd>

<dt id="track-email">Can I use GoatCounter to track if someone opened an email? <a href="#track-email">§</a></dt>
<dd>
Kind of but not really. You can include the tracking pixel in the email as an
image <a href="{{.Base}}/help/pixel">as described here</a>.

But this doesn't really work because almost all email clients block external
images by default exactly to prevent this sort of thing from working. All email
trackers “work” like this and none of them really work.

And in my personal opinion this is also where “statistics about visitors” turns
in to “invasive tracking” and “spying”, so consider if you <em>really</em> need
and want to do this in the first place. I have no way to prevent you from doing
this, but it’s also not a recommended or supported use case.
</dd>

<dt id="verify-email">Why do I need to verify my email? <a href="#verify-email">§</a></dt>
<dd>
Having some means of contact is useful in case of questions, problems, or other
reasons for communicating.

For example, if you’re sending many (millions) of pageviews then I’d rather
contact you to discuss options than just shut down the account. Not having any
means to get in touch would leave me in an awkward position.

It’s not too uncommon that people fill in the wrong email address, and this is
the only way to verify it.
</dd>
</dl>

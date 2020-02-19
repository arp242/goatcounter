begin;
		delete from updates where subject='Outage 😞';
	insert into updates (subject, created_at, show_at, body) values (
		'Outage 😞', now(), now(),

		'
<p>For about 12 hours (from Feb 18 20:00 until Feb 19 09:00, UTC) GoatCounter
didn’t collect any pageviews 😞</p>

<p>The first mistake was a small update I pushed yesterday with some minor code refactors.
GoatCounter persists the pageviews in the background to reduce database load and ensure the
<code>/count</code> endpoint is always fast, but the background cron wasn’t being run so … nothing
got persisted to the database.</p>

<p>The fix was just two characters: <code>defer setupCron(db)</code> to <code>defer setupCron(db)()</code>.
It was a silly mistake.</p>

<p>This shouldn’t have resulted in any data loss, since Varnish (the HTTP proxy/load balancer) logs
all requests exactly to recover from this kind of thing. The second mistake is that the log files
would be truncated whenever Varnish restarts, instead of appended to them. I restarted Varnish just
before I discovered this to clear the cache after some frontend changes. I fixed this as well, but
it’s too late to recover the previous logs.</p>

<p>So unfortunately there is no way to recover from this and there’s a 12-hour gap in your pageviews
😞 I’m really sorry about this; it definitely ruined my day.</p>

<p>I’ll improve the monitoring to also send alerts if the number of pageviews drops dramatically.
I’ll also improve the integration testing (most of this code is tested already, but it’s not
a full integration test yet).</p>
');

	insert into version values ('2020-02-19-2-outage');
commit;


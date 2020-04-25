begin;
	delete from updates where subject='One-time donation option';
	insert into updates (subject, created_at, show_at, body) values (
		'One-time donation option', now(), now(), '
		<p>There is now a one-time donation option; several people have asked for
		this in the last few months, so here it is :-)</p>
		<p>The link is available from the billing page, or as a direct link:
		<a href="/billing/donate">/billing/donate</a></p>
	');

	insert into version values ('2020-04-25-1-donate');
commit;

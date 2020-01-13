begin;
	delete from updates where subject = 'GoatCounter 1.0 released';
	insert into updates (subject, created_at, show_at, body) values (
		'GoatCounter 1.0 released', now(), now(),
		'<p>I just tagged GoatCounter 1.0! There are no exciting sudden changes
			as updates are usually deployed immediately, but it is an important
			milestone in the development of GoatCounter: this is the first version
			that I consider complete and stable enough to “shout from the roofs”, as
			we say in Dutch :-)
			Also see the <a href="https://www.arp242.net/goatcounter-1.0.html">announcement post</a>.</p>

		<p>I aim to release a new version roughly every 6 to 8 weeks. The next
			version will mostly focus on small UX improvements and making the
			self-hosting experience better. You can see the
			<a href="https://github.com/zgoat/goatcounter/milestone/3">roadmap for 1.1 on GitHub</a>.
			These changes will be deployed when they’re ready rather than in one go.
			I’ll be providing some more updates in this space when I make
			user-visible changes, but for the full ChangeLog see the GitHub commit
			log.</p>

		<p><strong>Feedback is important</strong>, so let me know if there’s
			anything in particular you’re missing.</p>'
	);

	insert into version values ('2020-01-13-1-update');
commit;

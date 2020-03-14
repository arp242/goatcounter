begin;
	alter table sites
		drop constraint sites_plan_check;
	alter table sites
		add constraint sites_plan_check check(plan in ('personal', 'personalplus', 'business', 'businessplus', 'child', 'custom'));

	insert into updates (subject, created_at, show_at, body) values (
		'Personal plus plan and GitHub Sponsors', now(), now(),
		'<p>You can now contribute through the GitHub Sponsors as well; since
			GitHub will match contributions in the first year this is now the
			preferred method, since you’ll get more bang for your buck ;-)
			<a href="https://github.com/sponsors/arp242/">https://github.com/sponsors/arp242/</a>
		</p>

		<p>I also added a “Personal Plus” plan. Like the Personal plan, this is
			for non-commercial use only, but allows you to use a custom domain
			with GoatCounter; e.g. stats.mydomain.com instead of
			mine.goatcounter.com. This is €5/month.</p>
	');

	insert into version values ('2020-02-19-1-personalplus');
commit;

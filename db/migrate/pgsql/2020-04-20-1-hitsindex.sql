begin;
	-- Note this requires a new session (i.e. server restart) to take effect.
	DO $$
		BEGIN
			execute 'alter database ' || current_database() || ' set random_page_cost=0.5';
			execute 'alter database ' || current_database() || ' set seq_page_cost=2';
		END
	$$;

	insert into version values ('2020-04-20-1-hitsindex');
commit;

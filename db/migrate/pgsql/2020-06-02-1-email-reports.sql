begin;
	alter table users add column last_report_at timestamp default null;

	create or replace function percent_diff(start float4, final float4) returns float4 as $$
		begin
			return case
				when start=0 then float4 '+infinity'
				else              (final - start) / start * 100
			end;
		end; $$
		language plpgsql
		immutable
		strict;

	insert into version values ('2020-06-02-1-email-reports');
commit;

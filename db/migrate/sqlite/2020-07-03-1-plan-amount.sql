begin;
	alter table sites add column billing_amount varchar;

	insert into version values('2020-07-03-1-plan-amount');
commit;

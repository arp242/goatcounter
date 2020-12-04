begin;
	-- Correct some data while we're at it.
	update hits set path = regexp_replace(path, '\?__cf_chl_captcha_tk__=.*?(&|$)', '')
		where path like '%?__cf_chl_captcha_tk__=%';

	update hits set path = regexp_replace(path, '\?__cf_chl_jschl_tk__=.*?(&|$)', '')
		where path like '%?__cf_chl_jschl_tk__=%';

	update hits set ref='Baidu', ref_scheme='g' where
		ref like 'baidu.com/%' or
		ref like 'c.tieba.baidu.com/%' or
		ref like 'm.baidu.com/%' or
		ref like 'tieba.baidu.com/%' or
		ref like 'www.baidu.com/%';

	update hits set ref='Yahoo', ref_scheme='g' where ref like '%search.yahoo.com%';

	insert into versions values('2020-08-28-7-refs');
commit;

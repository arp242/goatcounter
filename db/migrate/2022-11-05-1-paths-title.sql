drop index "paths#path#title";
create index "paths#title" on paths(lower(title));

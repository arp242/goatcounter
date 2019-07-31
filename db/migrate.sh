#!/bin/sh
#
# SQLite doesn't allow stuff like removing columns, so export and import
# everything.
#

set -euC

: >| data.sql
sqlite3 ./old.sqlite3 '.headers on' '.mode insert' 'select * from sites' |
	sed -E 's/INSERT INTO "table"/INSERT INTO "sites"/; s/name,//;'"s/VALUES\((.),''/VALUES(\1/" |
	sed "s/'2019-07-31'/'2019-07-31 05:05:05'/; s/'2019-07-30 00:08:08.356615811+00:00'/'2019-07-30 00:08:08'/" |
	sed -E 's/;$/,/; $s/,$/;/; 2,$s/INSERT INTO .*? VALUES//; 1s/ VALUES/ VALUES\n/;' \
	>> data.sql
 sqlite3 ./old.sqlite3 '.headers on' '.mode insert' 'select * from users' |
 	sed 's/INSERT INTO "table"/INSERT INTO "users"/' |
	sed -E 's/;$/,/; $s/,$/;/; 2,$s/INSERT INTO .*? VALUES//; 1s/ VALUES/ VALUES\n/;' \
 	>> data.sql
sqlite3 ./old.sqlite3 '.headers on' '.mode insert' 'select * from hits' |
	sed -E 's/INSERT INTO "table"/INSERT INTO "hits"/;'"s/\.[[:digit:]]+\+00:00//" |
	sed -E 's/;$/,/; $s/,$/;/; 2,$s/INSERT INTO .*? VALUES//; 1s/ VALUES/ VALUES\n/;' \
	>> data.sql
sqlite3 ./old.sqlite3 '.headers on' '.mode insert' 'select * from hit_stats' |
	sed 's/INSERT INTO "table"/INSERT INTO "hit_stats"/; s/kind,//;'"s/'h',//;" |
	sed -E 's/;$/,/; $s/,$/;/; 2,$s/INSERT INTO .*? VALUES//; 1s/ VALUES/ VALUES\n/;' \
 	>> data.sql
sqlite3 ./old.sqlite3 '.headers on' '.mode insert' 'select * from browser_stats' |
	sed -E 's/INSERT INTO "table"/INSERT INTO "browsers"/;'"s/\.[[:digit:]]+\+00:00//" |
	sed -E 's/;$/,/; $s/,$/;/; 2,$s/INSERT INTO .*? VALUES//; 1s/ VALUES/ VALUES\n/;' \
 	>> data.sql

sqlite3 new.sqlite3 < schema.sql
sqlite3 new.sqlite3 < data.sql
sqlite3 new.sqlite3 'vacuum;'

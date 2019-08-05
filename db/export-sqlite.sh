#!/bin/sh
#
# Export data from SQLite.
#
# Recreate database:
#
#   sqlite3 new.sqlite3 < schema.sql
#   ./export-sqlite.sh old.sqlite3 | sqlite3 new.sqlite3
#
# Convert data to PostgreSQL:
#
#   createdb goatcounter
#   psql goatcounter -c '\i schema.pgsql'
#   ./export-sqlite.sh goatcounter.sqlite3 | psql goatcounter
#

set -euC

db="$1"

exp() {
	local tbl="$1"
	sqlite3 "$db" '.headers on' '.mode insert' "select * from $tbl" |
		sed -E 's/INSERT INTO "table"/INSERT INTO "'"$tbl"'" /' |
		sed -E 's/;$/,/; $s/,$/;/; 2,$s/INSERT INTO .*? VALUES//; 1s/ VALUES\(/ VALUES\n\t(/; s/^\(/\t(/;' |
		./debin.py
	echo
}

exp 'sites'
exp 'users'
exp 'hits'
exp 'hit_stats'
exp 'browsers'

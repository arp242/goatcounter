This sets up a basic GoatCounter installation on Alpine Linux, using SQLite.

This is also available as a ["StackScript" for Linode][s]; If you don't have a
Linode account yet then [consider using my "referral URL"][r] and I'll get some
cash back from Linode :-)

It should be fine to run this more than once; and can be used to upgrade to a
newer version.

You can set the version to use with `GOATCOUNTER_VERSION`; this needs to be a
release on GitHub:

    $ GOATCOUNTER_VERSION=v2.0.0 ./goatcounter-alpine.sh

You can create additional sites with:

    $ cd /home/goatcounter
    $ ./bin/goatcounter db create site -vhost example.com -user.email me@example.com

Files are stored in `/home/goatcounter`; see `/var/log/goatcounter/current` for
logs; and you can configure the flags in `/etc/conf.d/goatcounter`

[s]: https://cloud.linode.com/stackscripts/659823
[r]: https://www.linode.com/?r=7acaf75737436d859e785dd5c9abe1ae99b4387e

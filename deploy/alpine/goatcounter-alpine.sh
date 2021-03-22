#!/bin/sh
# <UDF name="goatcounter_domain"   label="Domain you'll be hosting GoatCounter on" example="stats.example.com" />
# <UDF name="goatcounter_email"    label="Your email address"                      example="me@example.com" />
# <UDF name="goatcounter_password" label="Password to access GoatCounter"          example="Password 1234 :-)" />
# <UDF name="goatcounter_version"  label="GoatCounter version"                     default="v2.0.0" />
#
# This will set up an Alpine Linux machine; environment variables:
#    GOATCOUNTER_DOMAIN      Domain you'll be hosting GoatCounter on
#    GOATCOUNTER_EMAIL       Your email address
#    GOATCOUNTER_PASSWORD    Password to access GoatCounter
#    GOATCOUNTER_VERSION     GoatCounter version (default: v2.0.0).
#
# This is available as a "StackScript" to deploy GoatCounter on a Linode VPS:
# https://cloud.linode.com/stackscripts/659823
#
# If you don't have a Linode account yet then consider using my "referral URL"
# and I'll get some cash back from Linode :-)
# https://www.linode.com/?r=7acaf75737436d859e785dd5c9abe1ae99b4387e
#
# This script's source at the GoatCounter repo is:
# https://github.com/zgoat/goatcounter/blob/master/deploy/alpine
#
# It should be fine to run this more than once; and can be used to upgrade to a
# newer version.
#
# Files are stored in /home/goatcounter; see /var/log/goatcounter for logs; and
# you can configure the flags in /etc/conf.d/goatcounter.
#
# You can create additional sites with
#
#     $ cd /home/goatcounter
#     $ ./bin/goatcounter db create site [..]
#
# Please report any bugs, problems, or other issues on the GoatCounter issue
# tracker, or email me at martin@goatcounter.com
#

# GoatCounter version to set up.
v=${GOATCOUNTER_VERSION:-"v2.0.0"}


set -eu

if [ -z "${GOATCOUNTER_DOMAIN:-}" ]; then
	printf 2>&1 'Must set a domain; see the script header for usage.\n'
	exit 1
fi
if [ -z "${GOATCOUNTER_EMAIL:-}" ]; then
	printf 2>&1 'Must set a email; see the script header for usage.\n'
	usage
	exit 1
fi
if [ -z "${GOATCOUNTER_PASSWORD:-}" ]; then
	printf 2>&1 'Must set a password; see the script header for usage.\n'
	usage
	exit 1
fi

# Required packages.
apk add tzdata

# Setup user and group.
grep -q '^goatcounter:' /etc/group  || addgroup -S goatcounter
grep -q '^goatcounter:' /etc/passwd || adduser -s /sbin/nologin -DS -G goatcounter goatcounter

# Get latest version if it doesn't exist yet.
mkdir -p /home/goatcounter/bin
dst="/home/goatcounter/bin/goatcounter-$v"
if [ ! -f "$dst" ]; then
	curl -L \
		"https://github.com/zgoat/goatcounter/releases/download/$v/goatcounter-$v-linux-amd64.gz" |
		gzip -d > "$dst"
fi
chmod a+x "$dst"
setcap 'cap_net_bind_service=+ep cap_sys_chroot=+ep' "$dst"
ln -sf "$dst" "/home/goatcounter/bin/goatcounter"

# Set up site; this may fail if the site already exists, which is fine.
cd /home/goatcounter
./bin/goatcounter db create site -createdb -vhost "$GOATCOUNTER_DOMAIN" -user.email "$GOATCOUNTER_EMAIL" -user.password "$GOATCOUNTER_PASSWORD" ||:
chown -R goatcounter:goatcounter db

# Set up log directory.
mkdir -p /var/log/goatcounter
chown goatcounter:goatcounter /var/log/goatcounter

# Install, enable, and start service.
cat << EOF > /etc/init.d/goatcounter
#!/sbin/openrc-run

name="goatcounter"
description="GoatCounter web analytics"

command=/home/goatcounter/bin/goatcounter
directory=/home/goatcounter
command_args="serve -listen \${GOATCOUNTER_LISTEN:-:443} -db \${GOATCOUNTER_DB:-sqlite://./db/goatcounter.sqlite3} \${GOATCOUNTER_OPTS:--automigrate}"
command_user="goatcounter:goatcounter"
command_background="yes"
pidfile="/run/\${RC_SVCNAME}.pid"
output_log="/var/log/\${RC_SVCNAME}/current"
error_log="/var/log/\${RC_SVCNAME}/current"

start_pre() {
	# Make sure this is correct after updates etc.
	setcap 'cap_net_bind_service=+ep cap_sys_chroot=+ep' "\$(readlink "\$command")"
}

depend() {
	use net
	use dns
	after firewall
}
EOF

cat << EOF > /etc/conf.d/goatcounter
# The uncommented values are the defaults.

# Listen on all addresses.
#GOATCOUNTER_LISTEN=:443

# Location of SQLite3 database file or PostgreSQL connection. GoatCounter is
# started from /home/goatcounter.
#GOATCOUNTER_DB="sqlite://./db/goatcounter.sqlite3"

# If you use PostgreSQL then URI-type connector is recommended, as OpenRC can't
# deal well with spaces; for example:
#GOATCOUNTER_DB="postgresql:///run/postgresql/goatcounter?sslmode=disable"


# Other flags to add. See "goatcounter help serve".
#GOATCOUNTER_ARGS="-automigrate"
EOF

chmod a+x /etc/init.d/goatcounter
rc-update add goatcounter ||:
rc-service goatcounter start

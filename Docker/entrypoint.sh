#!/usr/bin/env bash

set -eu

create_site ()
{
  goatcounter db create site \
    -createdb \
    -domain "$GOATCOUNTER_DOMAIN" \
    -user.email "$GOATCOUNTER_EMAIL" \
    -password "$GOATCOUNTER_PASSWORD" \
    -db "$GOATCOUNTER_DB"
}

# silence any errors
if ! create_site; then
  /bin/true
fi

exec "$@"

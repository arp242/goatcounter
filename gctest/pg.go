//go:build testpg

package gctest

import (
	_ "zgo.at/zdb-drivers/pgx"
)

func init() {
	pgSQL = true
}

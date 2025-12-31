//go:build testpg

package gctest

import (
	_ "zgo.at/zdb-drivers/pq"
)

func init() {
	pgSQL = true
}

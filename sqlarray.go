package goatcounter

import (
	"database/sql/driver"
	"encoding/json"

	"github.com/lib/pq"
	"zgo.at/goatcounter/cfg"
)

type SQLArray []string

// Value determines what to store in the DB.
func (l SQLArray) Value() (driver.Value, error) {
	if cfg.PgSQL {
		return pq.Array(l), nil
	}
	return json.Marshal(l)
}

// Scan converts the data from the DB.
//func (l *SQLArray) Scan(v interface{}) error {
//	if v == nil {
//		return nil
//	}
//	strs := []string{}
//	for _, s := range strings.Split(fmt.Sprintf("%s", v), ",") {
//		s = strings.TrimSpace(s)
//		if s == "" {
//			continue
//		}
//		strs = append(strs, s)
//	}
//	*l = strs
//	return nil
//}

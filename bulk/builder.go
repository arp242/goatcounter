package bulk

import (
	"fmt"
	"strings"
)

type builder struct {
	table string
	cols  []string
	vals  [][]interface{}
}

func newBuilder(table string, cols ...string) builder {
	return builder{table: table, cols: cols}
}

func (b *builder) values(vals ...interface{}) {
	b.vals = append(b.vals, vals)
}

func (b *builder) SQL(vals ...string) (string, []interface{}) {
	var s strings.Builder
	s.WriteString("insert into ")
	s.WriteString(b.table)
	s.WriteString(" (")

	s.WriteString(strings.Join(b.cols, ","))
	s.WriteString(") values ")

	offset := 0
	var args []interface{}
	for i := range b.vals {
		s.WriteString("(")
		for j := range b.vals[i] {
			offset++
			s.WriteString(fmt.Sprintf("$%d", offset))
			if j < len(b.vals[i])-1 {
				s.WriteString(",")
			}
			args = append(args, b.vals[i][j])
		}
		s.WriteString(")")
		if i < len(b.vals)-1 {
			s.WriteString(",")
		}
	}

	return s.String(), args
}

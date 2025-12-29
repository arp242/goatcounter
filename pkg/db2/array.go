package db2

import (
	"context"
	"strconv"
	"strings"

	"zgo.at/zdb"
)

func In(ctx context.Context) zdb.SQL {
	if zdb.SQLDialect(ctx) == zdb.DialectPostgreSQL {
		return "= any"
	}
	return "in"
}

func NotIn(ctx context.Context, col string) zdb.SQL {
	if zdb.SQLDialect(ctx) == zdb.DialectPostgreSQL {
		return zdb.SQL("not " + col + " = any")
	}
	return zdb.SQL("not in " + col)
}

func Array[T ~int8 | ~int16 | ~int32 | ~int64](ctx context.Context, p []T) any {
	if zdb.SQLDialect(ctx) == zdb.DialectSQLite {
		return p
	}

	if len(p) == 0 {
		var zero T
		return zero
	}

	var b strings.Builder
	b.Grow(len(p) * 3)
	b.WriteByte('{')
	for i, pp := range p {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(strconv.FormatInt(int64(pp), 10))
	}
	b.WriteByte('}')
	return b.String()
}

var sqlArrayEscaper = strings.NewReplacer(`'`, `''`, `"`, `\"`)

func ArrayString(ctx context.Context, p []string) any {
	if zdb.SQLDialect(ctx) == zdb.DialectSQLite {
		return p
	}
	if len(p) == 0 {
		return ""
	}

	b := new(strings.Builder)
	b.Grow(len(p) * 3)
	b.WriteString("'{")
	for i, pp := range p {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteByte('"')
		sqlArrayEscaper.WriteString(b, pp)
		b.WriteByte('"')
	}
	b.WriteString("}'")
	return zdb.SQL(b.String())
}

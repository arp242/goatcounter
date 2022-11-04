package goatcounter

import (
	"context"
	"fmt"
	"strconv"

	"zgo.at/errors"
	"zgo.at/zcache"
	"zgo.at/zdb"
)

type Size struct {
	ID     int64   `db:"size_id"`
	Width  int16   `db:"width"`
	Height int16   `db:"height"`
	Scale  float64 `db:"scale"`
	Size   string  `db:"size"`
}

func (s *Size) Defaults(ctx context.Context)       {}
func (s *Size) Validate(ctx context.Context) error { return nil }

func (s Size) String() string {
	return strconv.Itoa(int(s.Width)) + "," + strconv.Itoa(int(s.Height)) +
		"," + strconv.FormatFloat(s.Scale, 'f', 15, 64)
}

func (s *Size) GetOrInsert(ctx context.Context, size Floats) error {
	k := fmt.Sprintf("%v", size)
	c, ok := cacheSizes(ctx).Get(k)
	if ok {
		*s = c.(Size)
		cacheSizes(ctx).Touch(k, zcache.DefaultExpiration)
		return nil
	}

	// Sometimes people send invalid values; don't error out, just set as
	// unknown size.
	if len(size) != 3 {
		s.ID = 0
		cacheSizes(ctx).SetDefault(k, *s)
		return nil
	}

	s.Width, s.Height, s.Scale = int16(size[0]), int16(size[1]), size[2]

	err := zdb.Get(ctx, s, `/* Size.GetOrInsert */
		select * from sizes where size = ? limit 1`, s.String())
	if err == nil {
		cacheSizes(ctx).SetDefault(k, *s)
		return nil
	}
	if !zdb.ErrNoRows(err) {
		return errors.Wrap(err, "Size.GetOrInsert get")
	}

	s.ID, err = zdb.InsertID(ctx, "size_id",
		`insert into sizes (width, height, scale) values (?, ?, ?)`,
		s.Width, s.Height, s.Scale)
	if err != nil {
		return errors.Wrap(err, "Size.GetOrInsert insert")
	}

	cacheSizes(ctx).SetDefault(k, *s)
	return nil
}

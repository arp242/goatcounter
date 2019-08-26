package bulk_test

import (
	"testing"

	"github.com/jmoiron/sqlx"
	"zgo.at/goatcounter"
	"zgo.at/goatcounter/bulk"
)

func TestInsert(t *testing.T) {
	ctx, clean := goatcounter.StartTest(t)
	defer clean()

	db := goatcounter.MustGetDB(ctx).(*sqlx.DB)
	_, err := db.Exec(`create table TBL (aa text, bb text, cc text);`)
	if err != nil {
		t.Fatal(err)
	}

	insert := bulk.NewInsert(ctx, db, "TBL", []string{"aa", "bb", "cc"})
	insert.Values("one", "two", "three")
	insert.Values("a", "b", "c")

	err = insert.Finish()
	if err != nil {
		t.Fatal(err)
	}
}

package goatcounter

import (
	"testing"
)

func TestBegin(t *testing.T) {
	ctx, clean := StartTest(t)
	defer clean()

	txctx, tx, err := Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}

	err = tx.Rollback()
	if err != nil {
		t.Fatal(err)
	}

	t.Run("nested", func(t *testing.T) {
		// Just ensure it won't panic. Nested transactions aren't supported yet.
		_, _, err = Begin(txctx)
		if err != nil {
			t.Fatal(err)
		}
	})
}

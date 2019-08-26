package bulk

import (
	"reflect"
	"testing"
)

func TestBuilder(t *testing.T) {
	b := newBuilder("TBL", "col1", "col2", "col3")
	b.values("one", "two", "three")
	b.values("a", "b", "c")

	want := `insert into TBL (col1,col2,col3) values ($1,$2,$3),($4,$5,$6)`
	wantargs := []interface{}{"one", "two", "three", "a", "b", "c"}

	query, args := b.SQL()
	if query != want {
		t.Errorf("wrong query\nwant: %q\ngot:  %q", want, query)
	}
	if !reflect.DeepEqual(args, wantargs) {
		t.Errorf("wrong args\nwant: %q\ngot:  %q", wantargs, args)
	}
}

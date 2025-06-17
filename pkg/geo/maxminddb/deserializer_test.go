package maxminddb

import (
	"math/big"
	"net/netip"
	"testing"
)

func TestDecodingToDeserializer(t *testing.T) {
	reader, err := Open(testFile("MaxMind-DB-test-decoder.mmdb"))
	if err != nil {
		t.Fatal(err)
	}

	dser := testDeserializer{}
	err = reader.Lookup(netip.MustParseAddr("::1.1.1.0")).Decode(&dser)
	if err != nil {
		t.Fatal(err)
	}

	checkDecodingToInterface(t, dser.rv)
}

type stackValue struct {
	value  any
	curNum int
}

type testDeserializer struct {
	stack []*stackValue
	rv    any
	key   *string
}

func (*testDeserializer) ShouldSkip(_ uintptr) (bool, error) {
	return false, nil
}

func (d *testDeserializer) StartSlice(size uint) error {
	return d.add(make([]any, size))
}

func (d *testDeserializer) StartMap(_ uint) error {
	return d.add(map[string]any{})
}

//nolint:unparam // This is to meet the requirements of the interface.
func (d *testDeserializer) End() error {
	d.stack = d.stack[:len(d.stack)-1]
	return nil
}

func (d *testDeserializer) String(v string) error {
	return d.add(v)
}

func (d *testDeserializer) Float64(v float64) error {
	return d.add(v)
}

func (d *testDeserializer) Bytes(v []byte) error {
	return d.add(v)
}

func (d *testDeserializer) Uint16(v uint16) error {
	return d.add(uint64(v))
}

func (d *testDeserializer) Uint32(v uint32) error {
	return d.add(uint64(v))
}

func (d *testDeserializer) Int32(v int32) error {
	return d.add(int(v))
}

func (d *testDeserializer) Uint64(v uint64) error {
	return d.add(v)
}

func (d *testDeserializer) Uint128(v *big.Int) error {
	return d.add(v)
}

func (d *testDeserializer) Bool(v bool) error {
	return d.add(v)
}

func (d *testDeserializer) Float32(v float32) error {
	return d.add(v)
}

func (d *testDeserializer) add(v any) error {
	if len(d.stack) == 0 {
		d.rv = v
	} else {
		top := d.stack[len(d.stack)-1]
		switch parent := top.value.(type) {
		case map[string]any:
			if d.key == nil {
				key := v.(string)
				d.key = &key
			} else {
				parent[*d.key] = v
				d.key = nil
			}

		case []any:
			parent[top.curNum] = v
			top.curNum++
		default:
		}
	}

	switch v := v.(type) {
	case map[string]any, []any:
		d.stack = append(d.stack, &stackValue{value: v})
	default:
	}

	return nil
}

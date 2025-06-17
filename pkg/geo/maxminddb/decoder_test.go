package maxminddb

import (
	"encoding/hex"
	"math/big"
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestBool(t *testing.T) {
	bools := map[string]any{
		"0007": false,
		"0107": true,
	}

	validateDecoding(t, bools)
}

func TestDouble(t *testing.T) {
	doubles := map[string]any{
		"680000000000000000": 0.0,
		"683FE0000000000000": 0.5,
		"68400921FB54442EEA": 3.14159265359,
		"68405EC00000000000": 123.0,
		"6841D000000007F8F4": 1073741824.12457,
		"68BFE0000000000000": -0.5,
		"68C00921FB54442EEA": -3.14159265359,
		"68C1D000000007F8F4": -1073741824.12457,
	}
	validateDecoding(t, doubles)
}

func TestFloat(t *testing.T) {
	floats := map[string]any{
		"040800000000": float32(0.0),
		"04083F800000": float32(1.0),
		"04083F8CCCCD": float32(1.1),
		"04084048F5C3": float32(3.14),
		"0408461C3FF6": float32(9999.99),
		"0408BF800000": float32(-1.0),
		"0408BF8CCCCD": float32(-1.1),
		"0408C048F5C3": -float32(3.14),
		"0408C61C3FF6": float32(-9999.99),
	}
	validateDecoding(t, floats)
}

func TestInt32(t *testing.T) {
	int32s := map[string]any{
		"0001":         0,
		"0401ffffffff": -1,
		"0101ff":       255,
		"0401ffffff01": -255,
		"020101f4":     500,
		"0401fffffe0c": -500,
		"0201ffff":     65535,
		"0401ffff0001": -65535,
		"0301ffffff":   16777215,
		"0401ff000001": -16777215,
		"04017fffffff": 2147483647,
		"040180000001": -2147483647,
	}
	validateDecoding(t, int32s)
}

func TestMap(t *testing.T) {
	maps := map[string]any{
		"e0":                             map[string]any{},
		"e142656e43466f6f":               map[string]any{"en": "Foo"},
		"e242656e43466f6f427a6843e4baba": map[string]any{"en": "Foo", "zh": "人"},
		"e1446e616d65e242656e43466f6f427a6843e4baba": map[string]any{
			"name": map[string]any{"en": "Foo", "zh": "人"},
		},
		"e1496c616e677561676573020442656e427a68": map[string]any{
			"languages": []any{"en", "zh"},
		},
	}
	validateDecoding(t, maps)
}

func TestSlice(t *testing.T) {
	slice := map[string]any{
		"0004":                 []any{},
		"010443466f6f":         []any{"Foo"},
		"020443466f6f43e4baba": []any{"Foo", "人"},
	}
	validateDecoding(t, slice)
}

var testStrings = makeTestStrings()

func makeTestStrings() map[string]any {
	str := map[string]any{
		"40":       "",
		"4131":     "1",
		"43E4BABA": "人",
		"5b313233343536373839303132333435363738393031323334353637":         "123456789012345678901234567",
		"5c31323334353637383930313233343536373839303132333435363738":       "1234567890123456789012345678",
		"5d003132333435363738393031323334353637383930313233343536373839":   "12345678901234567890123456789",
		"5d01313233343536373839303132333435363738393031323334353637383930": "123456789012345678901234567890",
	}

	for k, v := range map[string]int{"5e00d7": 500, "5e06b3": 2000, "5f001053": 70000} {
		key := k + strings.Repeat("78", v)
		str[key] = strings.Repeat("x", v)
	}

	return str
}

func TestString(t *testing.T) {
	validateDecoding(t, testStrings)
}

func TestByte(t *testing.T) {
	b := make(map[string]any)
	for key, val := range testStrings {
		oldCtrl, err := hex.DecodeString(key[0:2])
		if err != nil {
			t.Fatal(err)
		}
		newCtrl := []byte{oldCtrl[0] ^ 0xc0}
		key = strings.Replace(key, hex.EncodeToString(oldCtrl), hex.EncodeToString(newCtrl), 1)
		b[key] = []byte(val.(string))
	}

	validateDecoding(t, b)
}

func TestUint16(t *testing.T) {
	uint16s := map[string]any{
		"a0":     uint64(0),
		"a1ff":   uint64(255),
		"a201f4": uint64(500),
		"a22a78": uint64(10872),
		"a2ffff": uint64(65535),
	}
	validateDecoding(t, uint16s)
}

func TestUint32(t *testing.T) {
	uint32s := map[string]any{
		"c0":         uint64(0),
		"c1ff":       uint64(255),
		"c201f4":     uint64(500),
		"c22a78":     uint64(10872),
		"c2ffff":     uint64(65535),
		"c3ffffff":   uint64(16777215),
		"c4ffffffff": uint64(4294967295),
	}
	validateDecoding(t, uint32s)
}

func TestUint64(t *testing.T) {
	ctrlByte := "02"
	bits := uint64(64)

	uints := map[string]any{
		"00" + ctrlByte:          uint64(0),
		"02" + ctrlByte + "01f4": uint64(500),
		"02" + ctrlByte + "2a78": uint64(10872),
	}
	for i := uint64(0); i <= bits/8; i++ {
		expected := uint64((1 << (8 * i)) - 1)

		input := hex.EncodeToString([]byte{byte(i)}) + ctrlByte + strings.Repeat("ff", int(i))
		uints[input] = expected
	}

	validateDecoding(t, uints)
}

// Dedup with above somehow.
func TestUint128(t *testing.T) {
	ctrlByte := "03"
	bits := uint(128)

	uints := map[string]any{
		"00" + ctrlByte:          big.NewInt(0),
		"02" + ctrlByte + "01f4": big.NewInt(500),
		"02" + ctrlByte + "2a78": big.NewInt(10872),
	}
	for i := uint(1); i <= bits/8; i++ {
		expected := powBigInt(big.NewInt(2), 8*i)
		expected = expected.Sub(expected, big.NewInt(1))
		input := hex.EncodeToString([]byte{byte(i)}) + ctrlByte + strings.Repeat("ff", int(i))

		uints[input] = expected
	}

	validateDecoding(t, uints)
}

// No pow or bit shifting for big int, apparently :-(
// This is _not_ meant to be a comprehensive power function.
func powBigInt(bi *big.Int, pow uint) *big.Int {
	newInt := big.NewInt(1)
	for range pow {
		newInt.Mul(newInt, bi)
	}
	return newInt
}

func validateDecoding(t *testing.T, tests map[string]any) {
	for inputStr, expected := range tests {
		inputBytes, err := hex.DecodeString(inputStr)
		if err != nil {
			t.Fatal(err)
		}
		d := decoder{buffer: inputBytes}

		var result any
		_, err = d.decode(0, reflect.ValueOf(&result), 0)
		if err != nil {
			t.Fatal(err)
		}

		if !reflect.DeepEqual(result, expected) {
			// A big case statement would produce nicer errors
			t.Errorf("Output was incorrect: %s  %s", inputStr, expected)
		}
	}
}

func TestPointers(t *testing.T) {
	bytes, err := os.ReadFile(testFile("maps-with-pointers.raw"))
	if err != nil {
		t.Fatal(err)
	}
	d := decoder{buffer: bytes}

	expected := map[uint]map[string]string{
		0:  {"long_key": "long_value1"},
		22: {"long_key": "long_value2"},
		37: {"long_key2": "long_value1"},
		50: {"long_key2": "long_value2"},
		55: {"long_key": "long_value1"},
		57: {"long_key2": "long_value2"},
	}

	for offset, expectedValue := range expected {
		var actual map[string]string
		_, err := d.decode(offset, reflect.ValueOf(&actual), 0)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(actual, expectedValue) {
			t.Errorf("Decode for pointer at %d failed", offset)
		}
	}
}

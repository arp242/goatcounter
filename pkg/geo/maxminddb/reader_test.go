package maxminddb

import (
	"errors"
	"fmt"
	"math/big"
	"math/rand"
	"net"
	"net/netip"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestReader(t *testing.T) {
	for _, recordSize := range []uint{24, 28, 32} {
		for _, ipVersion := range []uint{4, 6} {
			fileName := fmt.Sprintf("MaxMind-DB-test-ipv%d-%d.mmdb", ipVersion, recordSize)
			t.Run(fileName, func(t *testing.T) {
				reader, err := Open(testFile(fileName))
				if err != nil {
					t.Fatal(err)
				}
				checkMetadata(t, reader, ipVersion, recordSize)

				if ipVersion == 4 {
					checkIpv4(t, reader)
				} else {
					checkIpv6(t, reader)
				}
			})
		}
	}
}

func TestReaderBytes(t *testing.T) {
	for _, recordSize := range []uint{24, 28, 32} {
		for _, ipVersion := range []uint{4, 6} {
			fileName := fmt.Sprintf(
				testFile("MaxMind-DB-test-ipv%d-%d.mmdb"),
				ipVersion,
				recordSize,
			)
			bytes, err := os.ReadFile(fileName)
			if err != nil {
				t.Fatal(err)
			}
			reader, err := FromBytes(bytes)
			if err != nil {
				t.Fatal(err)
			}

			checkMetadata(t, reader, ipVersion, recordSize)

			if ipVersion == 4 {
				checkIpv4(t, reader)
			} else {
				checkIpv6(t, reader)
			}
		}
	}
}

func TestLookupNetwork(t *testing.T) {
	bigInt := new(big.Int)
	bigInt.SetString("1329227995784915872903807060280344576", 10)
	decoderRecord := map[string]any{
		"array": []any{
			uint64(1),
			uint64(2),
			uint64(3),
		},
		"boolean": true,
		"bytes": []uint8{
			0x0,
			0x0,
			0x0,
			0x2a,
		},
		"double": 42.123456,
		"float":  float32(1.1),
		"int32":  -268435456,
		"map": map[string]any{
			"mapX": map[string]any{
				"arrayX": []any{
					uint64(0x7),
					uint64(0x8),
					uint64(0x9),
				},
				"utf8_stringX": "hello",
			},
		},
		"uint128":     bigInt,
		"uint16":      uint64(0x64),
		"uint32":      uint64(0x10000000),
		"uint64":      uint64(0x1000000000000000),
		"utf8_string": "unicode! ☯ - ♫",
	}

	tests := []struct {
		IP              netip.Addr
		DBFile          string
		ExpectedNetwork string
		ExpectedRecord  any
		ExpectedFound   bool
	}{
		{
			IP:              netip.MustParseAddr("1.1.1.1"),
			DBFile:          "MaxMind-DB-test-ipv6-32.mmdb",
			ExpectedNetwork: "1.0.0.0/8",
			ExpectedRecord:  nil,
			ExpectedFound:   false,
		},
		{
			IP:              netip.MustParseAddr("::1:ffff:ffff"),
			DBFile:          "MaxMind-DB-test-ipv6-24.mmdb",
			ExpectedNetwork: "::1:ffff:ffff/128",
			ExpectedRecord:  map[string]any{"ip": "::1:ffff:ffff"},
			ExpectedFound:   true,
		},
		{
			IP:              netip.MustParseAddr("::2:0:1"),
			DBFile:          "MaxMind-DB-test-ipv6-24.mmdb",
			ExpectedNetwork: "::2:0:0/122",
			ExpectedRecord:  map[string]any{"ip": "::2:0:0"},
			ExpectedFound:   true,
		},
		{
			IP:              netip.MustParseAddr("1.1.1.1"),
			DBFile:          "MaxMind-DB-test-ipv4-24.mmdb",
			ExpectedNetwork: "1.1.1.1/32",
			ExpectedRecord:  map[string]any{"ip": "1.1.1.1"},
			ExpectedFound:   true,
		},
		{
			IP:              netip.MustParseAddr("1.1.1.3"),
			DBFile:          "MaxMind-DB-test-ipv4-24.mmdb",
			ExpectedNetwork: "1.1.1.2/31",
			ExpectedRecord:  map[string]any{"ip": "1.1.1.2"},
			ExpectedFound:   true,
		},
		{
			IP:              netip.MustParseAddr("1.1.1.3"),
			DBFile:          "MaxMind-DB-test-decoder.mmdb",
			ExpectedNetwork: "1.1.1.0/24",
			ExpectedRecord:  decoderRecord,
			ExpectedFound:   true,
		},
		{
			IP:              netip.MustParseAddr("::ffff:1.1.1.128"),
			DBFile:          "MaxMind-DB-test-decoder.mmdb",
			ExpectedNetwork: "::ffff:1.1.1.0/120",
			ExpectedRecord:  decoderRecord,
			ExpectedFound:   true,
		},
		{
			IP:              netip.MustParseAddr("::1.1.1.128"),
			DBFile:          "MaxMind-DB-test-decoder.mmdb",
			ExpectedNetwork: "::101:100/120",
			ExpectedRecord:  decoderRecord,
			ExpectedFound:   true,
		},
		{
			IP:              netip.MustParseAddr("200.0.2.1"),
			DBFile:          "MaxMind-DB-no-ipv4-search-tree.mmdb",
			ExpectedNetwork: "::/64",
			ExpectedRecord:  "::0/64",
			ExpectedFound:   true,
		},
		{
			IP:              netip.MustParseAddr("::200.0.2.1"),
			DBFile:          "MaxMind-DB-no-ipv4-search-tree.mmdb",
			ExpectedNetwork: "::/64",
			ExpectedRecord:  "::0/64",
			ExpectedFound:   true,
		},
		{
			IP:              netip.MustParseAddr("0:0:0:0:ffff:ffff:ffff:ffff"),
			DBFile:          "MaxMind-DB-no-ipv4-search-tree.mmdb",
			ExpectedNetwork: "::/64",
			ExpectedRecord:  "::0/64",
			ExpectedFound:   true,
		},
		{
			IP:              netip.MustParseAddr("ef00::"),
			DBFile:          "MaxMind-DB-no-ipv4-search-tree.mmdb",
			ExpectedNetwork: "8000::/1",
			ExpectedRecord:  nil,
			ExpectedFound:   false,
		},
	}

	for _, test := range tests {
		t.Run(fmt.Sprintf("%s - %s", test.DBFile, test.IP), func(t *testing.T) {
			var record any
			reader, err := Open(testFile(test.DBFile))
			if err != nil {
				t.Fatal(err)
			}

			result := reader.Lookup(test.IP)
			noError(t, result.Err())
			equal(t, test.ExpectedFound, result.Found())
			equal(t, test.ExpectedNetwork, result.Prefix().String())

			noError(t, result.Decode(&record))
			equal(t, test.ExpectedRecord, record)
		})
	}
}

func TestDecodingToInterface(t *testing.T) {
	reader, err := Open(testFile("MaxMind-DB-test-decoder.mmdb"))
	if err != nil {
		t.Fatal(err)
	}

	var recordInterface any
	err = reader.Lookup(netip.MustParseAddr("::1.1.1.0")).Decode(&recordInterface)
	if err != nil {
		t.Fatal(err)
	}

	checkDecodingToInterface(t, recordInterface)
}

func TestMetadataPointer(t *testing.T) {
	_, err := Open(testFile("MaxMind-DB-test-metadata-pointers.mmdb"))
	if err != nil {
		t.Fatal(err)
	}
}

func checkDecodingToInterface(t *testing.T, recordInterface any) {
	record := recordInterface.(map[string]any)
	equal(t, []any{uint64(1), uint64(2), uint64(3)}, record["array"])
	equal(t, true, record["boolean"])
	equal(t, []byte{0x00, 0x00, 0x00, 0x2a}, record["bytes"])
	inEpsilon(t, 42.123456, record["double"], 1e-10)
	inEpsilon(t, float32(1.1), record["float"], 1e-5)
	equal(t, -268435456, record["int32"])
	equal(t,
		map[string]any{
			"mapX": map[string]any{
				"arrayX":       []any{uint64(7), uint64(8), uint64(9)},
				"utf8_stringX": "hello",
			},
		},
		record["map"],
	)

	equal(t, uint64(100), record["uint16"])
	equal(t, uint64(268435456), record["uint32"])
	equal(t, uint64(1152921504606846976), record["uint64"])
	equal(t, "unicode! ☯ - ♫", record["utf8_string"])
	bigInt := new(big.Int)
	bigInt.SetString("1329227995784915872903807060280344576", 10)
	equal(t, bigInt, record["uint128"])
}

type TestType struct {
	Array      []uint         `maxminddb:"array"`
	Boolean    bool           `maxminddb:"boolean"`
	Bytes      []byte         `maxminddb:"bytes"`
	Double     float64        `maxminddb:"double"`
	Float      float32        `maxminddb:"float"`
	Int32      int32          `maxminddb:"int32"`
	Map        map[string]any `maxminddb:"map"`
	Uint16     uint16         `maxminddb:"uint16"`
	Uint32     uint32         `maxminddb:"uint32"`
	Uint64     uint64         `maxminddb:"uint64"`
	Uint128    big.Int        `maxminddb:"uint128"`
	Utf8String string         `maxminddb:"utf8_string"`
}

func TestDecoder(t *testing.T) {
	reader, err := Open(testFile("MaxMind-DB-test-decoder.mmdb"))
	if err != nil {
		t.Fatal(err)
	}

	verify := func(result TestType) {
		equal(t, []uint{uint(1), uint(2), uint(3)}, result.Array)
		isTrue(t, result.Boolean)
		equal(t, []byte{0x00, 0x00, 0x00, 0x2a}, result.Bytes)
		inEpsilon(t, 42.123456, result.Double, 1e-10)
		inEpsilon(t, float32(1.1), result.Float, 1e-5)
		equal(t, int32(-268435456), result.Int32)

		equal(t,
			map[string]any{
				"mapX": map[string]any{
					"arrayX":       []any{uint64(7), uint64(8), uint64(9)},
					"utf8_stringX": "hello",
				},
			},
			result.Map,
		)

		equal(t, uint16(100), result.Uint16)
		equal(t, uint32(268435456), result.Uint32)
		equal(t, uint64(1152921504606846976), result.Uint64)
		equal(t, "unicode! ☯ - ♫", result.Utf8String)
		bigInt := new(big.Int)
		bigInt.SetString("1329227995784915872903807060280344576", 10)
		equal(t, bigInt, &result.Uint128)
	}

	{
		// Directly lookup and decode.
		var testV TestType
		noError(t, reader.Lookup(netip.MustParseAddr("::1.1.1.0")).Decode(&testV))
		verify(testV)
	}
	{
		// Lookup record offset, then Decode.
		var testV TestType
		result := reader.Lookup(netip.MustParseAddr("::1.1.1.0"))
		noError(t, result.Err())
		isTrue(t, result.Found())

		res := reader.LookupOffset(result.Offset())
		noError(t, res.Decode(&testV))
		verify(testV)
	}

	noError(t, reader.Close())
}

func TestDecodePath(t *testing.T) {
	reader, err := Open(testFile("MaxMind-DB-test-decoder.mmdb"))
	if err != nil {
		t.Fatal(err)
	}

	result := reader.Lookup(netip.MustParseAddr("::1.1.1.0"))
	noError(t, result.Err())

	var u16 uint16

	noError(t, result.DecodePath(&u16, "uint16"))

	equal(t, uint16(100), u16)

	var u uint
	noError(t, result.DecodePath(&u, "array", 0))
	equal(t, uint(1), u)

	var u2 uint
	noError(t, result.DecodePath(&u2, "array", 2))
	equal(t, uint(3), u2)

	// This is past the end of the array
	var u3 uint
	noError(t, result.DecodePath(&u3, "array", 3))
	equal(t, uint(0), u3)

	// Negative offsets

	var n1 uint
	noError(t, result.DecodePath(&n1, "array", -1))
	equal(t, uint(3), n1)

	var n2 uint
	noError(t, result.DecodePath(&n2, "array", -3))
	equal(t, uint(1), n2)

	var u4 uint
	noError(t, result.DecodePath(&u4, "map", "mapX", "arrayX", 1))
	equal(t, uint(8), u4)

	// Does key not exist
	var ne uint
	noError(t, result.DecodePath(&ne, "does-not-exist", 1))
	equal(t, uint(0), ne)
}

type TestInterface interface {
	method() bool
}

func (t *TestType) method() bool {
	return t.Boolean
}

func TestStructInterface(t *testing.T) {
	var result TestInterface = &TestType{}

	reader, err := Open(testFile("MaxMind-DB-test-decoder.mmdb"))
	if err != nil {
		t.Fatal(err)
	}

	noError(t, reader.Lookup(netip.MustParseAddr("::1.1.1.0")).Decode(&result))

	isTrue(t, result.method())
}

func TestNonEmptyNilInterface(t *testing.T) {
	var result TestInterface

	reader, err := Open(testFile("MaxMind-DB-test-decoder.mmdb"))
	if err != nil {
		t.Fatal(err)
	}

	err = reader.Lookup(netip.MustParseAddr("::1.1.1.0")).Decode(&result)
	equal(
		t,
		"maxminddb: cannot unmarshal map into type maxminddb.TestInterface",
		err.Error(),
	)
}

type CityTraits struct {
	AutonomousSystemNumber uint `json:"autonomous_system_number,omitempty" maxminddb:"autonomous_system_number"`
}

type City struct {
	Traits CityTraits `maxminddb:"traits"`
}

func TestEmbeddedStructAsInterface(t *testing.T) {
	var city City
	var result any = city.Traits

	db, err := Open(testFile("GeoIP2-ISP-Test.mmdb"))
	if err != nil {
		t.Fatal(err)
	}

	noError(t, db.Lookup(netip.MustParseAddr("1.128.0.0")).Decode(&result))
}

type BoolInterface interface {
	true() bool
}

type Bool bool

func (b Bool) true() bool {
	return bool(b)
}

type ValueTypeTestType struct {
	Boolean BoolInterface `maxminddb:"boolean"`
}

func TestValueTypeInterface(t *testing.T) {
	var result ValueTypeTestType
	result.Boolean = Bool(false)

	reader, err := Open(testFile("MaxMind-DB-test-decoder.mmdb"))
	if err != nil {
		t.Fatal(err)
	}

	// although it would be nice to support cases like this, I am not sure it
	// is possible to do so in a general way.
	err = reader.Lookup(netip.MustParseAddr("::1.1.1.0")).Decode(&result)
	if err == nil {
		t.Fatal()
	}
}

type NestedMapX struct {
	UTF8StringX string `maxminddb:"utf8_stringX"`
}

type NestedPointerMapX struct {
	ArrayX []int `maxminddb:"arrayX"`
}

type PointerMap struct {
	MapX struct {
		Ignored string
		NestedMapX
		*NestedPointerMapX
	} `maxminddb:"mapX"`
}

type TestPointerType struct {
	Array   *[]uint     `maxminddb:"array"`
	Boolean *bool       `maxminddb:"boolean"`
	Bytes   *[]byte     `maxminddb:"bytes"`
	Double  *float64    `maxminddb:"double"`
	Float   *float32    `maxminddb:"float"`
	Int32   *int32      `maxminddb:"int32"`
	Map     *PointerMap `maxminddb:"map"`
	Uint16  *uint16     `maxminddb:"uint16"`
	Uint32  *uint32     `maxminddb:"uint32"`

	// Test for pointer to pointer
	Uint64     **uint64 `maxminddb:"uint64"`
	Uint128    *big.Int `maxminddb:"uint128"`
	Utf8String *string  `maxminddb:"utf8_string"`
}

func TestComplexStructWithNestingAndPointer(t *testing.T) {
	reader, err := Open(testFile("MaxMind-DB-test-decoder.mmdb"))
	if err != nil {
		t.Fatal(err)
	}

	var result TestPointerType

	err = reader.Lookup(netip.MustParseAddr("::1.1.1.0")).Decode(&result)
	if err != nil {
		t.Fatal(err)
	}

	equal(t, []uint{uint(1), uint(2), uint(3)}, *result.Array)
	isTrue(t, *result.Boolean)
	equal(t, []byte{0x00, 0x00, 0x00, 0x2a}, *result.Bytes)
	inEpsilon(t, 42.123456, *result.Double, 1e-10)
	inEpsilon(t, float32(1.1), *result.Float, 1e-5)
	equal(t, int32(-268435456), *result.Int32)

	equal(t, []int{7, 8, 9}, result.Map.MapX.ArrayX)

	equal(t, "hello", result.Map.MapX.UTF8StringX)

	equal(t, uint16(100), *result.Uint16)
	equal(t, uint32(268435456), *result.Uint32)
	equal(t, uint64(1152921504606846976), **result.Uint64)
	equal(t, "unicode! ☯ - ♫", *result.Utf8String)
	bigInt := new(big.Int)
	bigInt.SetString("1329227995784915872903807060280344576", 10)
	equal(t, bigInt, result.Uint128)

	noError(t, reader.Close())
}

// See GitHub #115.
func TestNestedMapDecode(t *testing.T) {
	db, err := Open(testFile("GeoIP2-Country-Test.mmdb"))
	if err != nil {
		t.Fatal(err)
	}

	var r map[string]map[string]any

	noError(t, db.Lookup(netip.MustParseAddr("89.160.20.128")).Decode(&r))

	equal(
		t,
		map[string]map[string]any{
			"continent": {
				"code":       "EU",
				"geoname_id": uint64(6255148),
				"names": map[string]any{
					"de":    "Europa",
					"en":    "Europe",
					"es":    "Europa",
					"fr":    "Europe",
					"ja":    "ヨーロッパ",
					"pt-BR": "Europa",
					"ru":    "Европа",
					"zh-CN": "欧洲",
				},
			},
			"country": {
				"geoname_id":           uint64(2661886),
				"is_in_european_union": true,
				"iso_code":             "SE",
				"names": map[string]any{
					"de":    "Schweden",
					"en":    "Sweden",
					"es":    "Suecia",
					"fr":    "Suède",
					"ja":    "スウェーデン王国",
					"pt-BR": "Suécia",
					"ru":    "Швеция",
					"zh-CN": "瑞典",
				},
			},
			"registered_country": {
				"geoname_id":           uint64(2921044),
				"is_in_european_union": true,
				"iso_code":             "DE",
				"names": map[string]any{
					"de":    "Deutschland",
					"en":    "Germany",
					"es":    "Alemania",
					"fr":    "Allemagne",
					"ja":    "ドイツ連邦共和国",
					"pt-BR": "Alemanha",
					"ru":    "Германия",
					"zh-CN": "德国",
				},
			},
		},
		r,
	)
}

func TestNestedOffsetDecode(t *testing.T) {
	db, err := Open(testFile("GeoIP2-City-Test.mmdb"))
	if err != nil {
		t.Fatal(err)
	}

	result := db.Lookup(netip.MustParseAddr("81.2.69.142"))
	noError(t, result.Err())
	isTrue(t, result.Found())

	var root struct {
		CountryOffset uintptr `maxminddb:"country"`

		Location struct {
			Latitude float64 `maxminddb:"latitude"`
			// Longitude is directly nested within the parent map.
			LongitudeOffset uintptr `maxminddb:"longitude"`
			// TimeZone is indirected via a pointer.
			TimeZoneOffset uintptr `maxminddb:"time_zone"`
		} `maxminddb:"location"`
	}
	res := db.LookupOffset(result.Offset())
	noError(t, res.Decode(&root))
	inEpsilon(t, 51.5142, root.Location.Latitude, 1e-10)

	var longitude float64
	res = db.LookupOffset(root.Location.LongitudeOffset)
	noError(t, res.Decode(&longitude))
	inEpsilon(t, -0.0931, longitude, 1e-10)

	var timeZone string
	res = db.LookupOffset(root.Location.TimeZoneOffset)
	noError(t, res.Decode(&timeZone))
	equal(t, "Europe/London", timeZone)

	var country struct {
		IsoCode string `maxminddb:"iso_code"`
	}
	res = db.LookupOffset(root.CountryOffset)
	noError(t, res.Decode(&country))
	equal(t, "GB", country.IsoCode)

	noError(t, db.Close())
}

func TestDecodingUint16IntoInt(t *testing.T) {
	reader, err := Open(testFile("MaxMind-DB-test-decoder.mmdb"))
	if err != nil {
		t.Fatal(err)
	}

	var result struct {
		Uint16 int `maxminddb:"uint16"`
	}
	err = reader.Lookup(netip.MustParseAddr("::1.1.1.0")).Decode(&result)
	if err != nil {
		t.Fatal(err)
	}

	equal(t, 100, result.Uint16)
}

func TestIpv6inIpv4(t *testing.T) {
	reader, err := Open(testFile("MaxMind-DB-test-ipv4-24.mmdb"))
	if err != nil {
		t.Fatal(err)
	}

	var result TestType
	err = reader.Lookup(netip.MustParseAddr("2001::")).Decode(&result)

	var emptyResult TestType
	equal(t, emptyResult, result)

	expected := errors.New(
		"error looking up '2001::': you attempted to look up an IPv6 address in an IPv4-only database",
	)
	equal(t, expected, err)
	noError(t, reader.Close())
}

func TestBrokenDoubleDatabase(t *testing.T) {
	reader, err := Open(testFile("GeoIP2-City-Test-Broken-Double-Format.mmdb"))
	if err != nil {
		t.Fatal(err)
	}

	var result any
	err = reader.Lookup(netip.MustParseAddr("2001:220::")).Decode(&result)

	wantErr := newInvalidDatabaseError("the MaxMind DB file's data section contains bad data (float 64 size of 2)")
	if !errors.As(err, &wantErr) {
		t.Errorf("wrong error\nwant: %#v\nhave: %#v", err, wantErr)
	}
	noError(t, reader.Close())
}

func TestInvalidNodeCountDatabase(t *testing.T) {
	_, err := Open(testFile("GeoIP2-City-Test-Invalid-Node-Count.mmdb"))

	expected := newInvalidDatabaseError("the MaxMind DB contains invalid metadata")
	equal(t, expected, err)
}

func TestMissingDatabase(t *testing.T) {
	reader, err := Open("file-does-not-exist.mmdb")
	if reader != nil {
		t.Error("received reader when doing lookups on DB that doesn't exist")
	}
	if err == nil || !strings.Contains(err.Error(), "open file-does-not-exist.mmdb") {
		t.Fatal()
	}
}

func TestNonDatabase(t *testing.T) {
	reader, err := Open("decoder.go")
	if reader != nil {
		t.Error("received reader when doing lookups on DB that doesn't exist")
	}
	equal(t, "error opening database: invalid MaxMind DB file", err.Error())
}

func TestDecodingToNonPointer(t *testing.T) {
	reader, err := Open(testFile("MaxMind-DB-test-decoder.mmdb"))
	if err != nil {
		t.Fatal(err)
	}

	var recordInterface any
	err = reader.Lookup(netip.MustParseAddr("::1.1.1.0")).Decode(recordInterface)
	equal(t, "result param must be a pointer", err.Error())
	noError(t, reader.Close())
}

// func TestNilLookup(t *testing.T) {
// 	reader, err := Open(testFile("MaxMind-DB-test-decoder.mmdb"))
// 	if err != nil { t.Fatal(err) }

// 	var recordInterface any
// 	err = reader.Lookup(nil).Decode( recordInterface)
// 	equal(t, "IP passed to Lookup cannot be nil", err.Error())
// 	noError(t, reader.Close(), "error on close")
// }

func TestUsingClosedDatabase(t *testing.T) {
	reader, err := Open(testFile("MaxMind-DB-test-decoder.mmdb"))
	if err != nil {
		t.Fatal(err)
	}
	noError(t, reader.Close())

	addr := netip.MustParseAddr("::")

	result := reader.Lookup(addr)
	equal(t, "cannot call Lookup on a closed database", result.Err().Error())

	var recordInterface any
	err = reader.Lookup(addr).Decode(recordInterface)
	equal(t, "cannot call Lookup on a closed database", err.Error())

	err = reader.LookupOffset(0).Decode(recordInterface)
	equal(t, "cannot call Decode on a closed database", err.Error())
}

func checkMetadata(t *testing.T, reader *Reader, ipVersion, recordSize uint) {
	metadata := reader.Metadata

	equal(t, uint(2), metadata.BinaryFormatMajorVersion)

	equal(t, uint(0), metadata.BinaryFormatMinorVersion)
	equal(t, "Test", metadata.DatabaseType)

	equal(t, map[string]string{
		"en": "Test Database",
		"zh": "Test Database Chinese",
	}, metadata.Description)
	equal(t, ipVersion, metadata.IPVersion)
	equal(t, []string{"en", "zh"}, metadata.Languages)

	if ipVersion == 4 {
		equal(t, uint(164), metadata.NodeCount)
	} else {
		equal(t, uint(416), metadata.NodeCount)
	}

	equal(t, recordSize, metadata.RecordSize)
}

func checkIpv4(t *testing.T, reader *Reader) {
	for i := range uint(6) {
		address := fmt.Sprintf("1.1.1.%d", uint(1)<<i)
		ip := netip.MustParseAddr(address)

		var result map[string]string
		err := reader.Lookup(ip).Decode(&result)
		if err != nil {
			t.Fatal(err)
		}
		equal(t, map[string]string{"ip": address}, result)
	}
	pairs := map[string]string{
		"1.1.1.3":  "1.1.1.2",
		"1.1.1.5":  "1.1.1.4",
		"1.1.1.7":  "1.1.1.4",
		"1.1.1.9":  "1.1.1.8",
		"1.1.1.15": "1.1.1.8",
		"1.1.1.17": "1.1.1.16",
		"1.1.1.31": "1.1.1.16",
	}

	for keyAddress, valueAddress := range pairs {
		data := map[string]string{"ip": valueAddress}

		ip := netip.MustParseAddr(keyAddress)

		var result map[string]string
		err := reader.Lookup(ip).Decode(&result)
		if err != nil {
			t.Fatal(err)
		}
		equal(t, data, result)
	}

	for _, address := range []string{"1.1.1.33", "255.254.253.123"} {
		ip := netip.MustParseAddr(address)

		var result map[string]string
		err := reader.Lookup(ip).Decode(&result)
		if err != nil {
			t.Fatal(err)
		}
		if result != nil {
			t.Fatal()
		}
	}
}

func checkIpv6(t *testing.T, reader *Reader) {
	subnets := []string{
		"::1:ffff:ffff", "::2:0:0",
		"::2:0:40", "::2:0:50", "::2:0:58",
	}

	for _, address := range subnets {
		var result map[string]string
		err := reader.Lookup(netip.MustParseAddr(address)).Decode(&result)
		if err != nil {
			t.Fatal(err)
		}
		equal(t, map[string]string{"ip": address}, result)
	}

	pairs := map[string]string{
		"::2:0:1":  "::2:0:0",
		"::2:0:33": "::2:0:0",
		"::2:0:39": "::2:0:0",
		"::2:0:41": "::2:0:40",
		"::2:0:49": "::2:0:40",
		"::2:0:52": "::2:0:50",
		"::2:0:57": "::2:0:50",
		"::2:0:59": "::2:0:58",
	}

	for keyAddress, valueAddress := range pairs {
		data := map[string]string{"ip": valueAddress}
		var result map[string]string
		err := reader.Lookup(netip.MustParseAddr(keyAddress)).Decode(&result)
		if err != nil {
			t.Fatal(err)
		}
		equal(t, data, result)
	}

	for _, address := range []string{"1.1.1.33", "255.254.253.123", "89fa::"} {
		var result map[string]string
		err := reader.Lookup(netip.MustParseAddr(address)).Decode(&result)
		if err != nil {
			t.Fatal(err)
		}
		if result != nil {
			t.Fatal()
		}
	}
}

func BenchmarkOpen(b *testing.B) {
	var db *Reader
	var err error
	for range b.N {
		db, err = Open("GeoLite2-City.mmdb")
		if err != nil {
			b.Fatal(err)
		}
	}
	if db != nil {
		b.Fail()
	}
	noError(b, db.Close())
}

func BenchmarkInterfaceLookup(b *testing.B) {
	db, err := Open("GeoLite2-City.mmdb")
	noError(b, err)

	//nolint:gosec // this is a test
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	var result any

	s := make(net.IP, 4)
	for range b.N {
		ip := randomIPv4Address(r, s)
		err = db.Lookup(ip).Decode(&result)
		if err != nil {
			b.Error(err)
		}
	}
	noError(b, db.Close())
}

func BenchmarkLookupNetwork(b *testing.B) {
	db, err := Open("GeoLite2-City.mmdb")
	noError(b, err)

	//nolint:gosec // this is a test
	r := rand.New(rand.NewSource(time.Now().UnixNano()))

	s := make(net.IP, 4)
	for range b.N {
		ip := randomIPv4Address(r, s)
		res := db.Lookup(ip)
		if err := res.Err(); err != nil {
			b.Error(err)
		}
		if !res.Prefix().IsValid() {
			b.Fatalf("invalid network for %s", ip)
		}
	}
	noError(b, db.Close())
}

type fullCity struct {
	City struct {
		GeoNameID uint              `maxminddb:"geoname_id"`
		Names     map[string]string `maxminddb:"names"`
	} `maxminddb:"city"`
	Continent struct {
		Code      string            `maxminddb:"code"`
		GeoNameID uint              `maxminddb:"geoname_id"`
		Names     map[string]string `maxminddb:"names"`
	} `maxminddb:"continent"`
	Country struct {
		GeoNameID         uint              `maxminddb:"geoname_id"`
		IsInEuropeanUnion bool              `maxminddb:"is_in_european_union"`
		IsoCode           string            `maxminddb:"iso_code"`
		Names             map[string]string `maxminddb:"names"`
	} `maxminddb:"country"`
	Location struct {
		AccuracyRadius uint16  `maxminddb:"accuracy_radius"`
		Latitude       float64 `maxminddb:"latitude"`
		Longitude      float64 `maxminddb:"longitude"`
		MetroCode      uint    `maxminddb:"metro_code"`
		TimeZone       string  `maxminddb:"time_zone"`
	} `maxminddb:"location"`
	Postal struct {
		Code string `maxminddb:"code"`
	} `maxminddb:"postal"`
	RegisteredCountry struct {
		GeoNameID         uint              `maxminddb:"geoname_id"`
		IsInEuropeanUnion bool              `maxminddb:"is_in_european_union"`
		IsoCode           string            `maxminddb:"iso_code"`
		Names             map[string]string `maxminddb:"names"`
	} `maxminddb:"registered_country"`
	RepresentedCountry struct {
		GeoNameID         uint              `maxminddb:"geoname_id"`
		IsInEuropeanUnion bool              `maxminddb:"is_in_european_union"`
		IsoCode           string            `maxminddb:"iso_code"`
		Names             map[string]string `maxminddb:"names"`
		Type              string            `maxminddb:"type"`
	} `maxminddb:"represented_country"`
	Subdivisions []struct {
		GeoNameID uint              `maxminddb:"geoname_id"`
		IsoCode   string            `maxminddb:"iso_code"`
		Names     map[string]string `maxminddb:"names"`
	} `maxminddb:"subdivisions"`
	Traits struct {
		IsAnonymousProxy    bool `maxminddb:"is_anonymous_proxy"`
		IsSatelliteProvider bool `maxminddb:"is_satellite_provider"`
	} `maxminddb:"traits"`
}

func BenchmarkCityLookup(b *testing.B) {
	db, err := Open("GeoLite2-City.mmdb")
	noError(b, err)

	//nolint:gosec // this is a test
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	var result fullCity

	s := make(net.IP, 4)
	for range b.N {
		ip := randomIPv4Address(r, s)
		err = db.Lookup(ip).Decode(&result)
		if err != nil {
			b.Error(err)
		}
	}
	noError(b, db.Close())
}

func BenchmarkCityLookupOnly(b *testing.B) {
	db, err := Open("GeoLite2-City.mmdb")
	noError(b, err)

	//nolint:gosec // this is a test
	r := rand.New(rand.NewSource(time.Now().UnixNano()))

	s := make(net.IP, 4)
	for range b.N {
		ip := randomIPv4Address(r, s)
		result := db.Lookup(ip)
		if err := result.Err(); err != nil {
			b.Error(err)
		}
	}
	noError(b, db.Close())
}

func BenchmarkDecodeCountryCodeWithStruct(b *testing.B) {
	db, err := Open("GeoLite2-City.mmdb")
	noError(b, err)

	type MinCountry struct {
		Country struct {
			IsoCode string `maxminddb:"iso_code"`
		} `maxminddb:"country"`
	}

	//nolint:gosec // this is a test
	r := rand.New(rand.NewSource(0))
	var result MinCountry

	s := make(net.IP, 4)
	for range b.N {
		ip := randomIPv4Address(r, s)
		err = db.Lookup(ip).Decode(&result)
		if err != nil {
			b.Error(err)
		}
	}
	noError(b, db.Close())
}

func BenchmarkDecodePathCountryCode(b *testing.B) {
	db, err := Open("GeoLite2-City.mmdb")
	noError(b, err)

	path := []any{"country", "iso_code"}

	//nolint:gosec // this is a test
	r := rand.New(rand.NewSource(0))
	var result string

	s := make(net.IP, 4)
	for range b.N {
		ip := randomIPv4Address(r, s)
		err = db.Lookup(ip).DecodePath(&result, path...)
		if err != nil {
			b.Error(err)
		}
	}
	noError(b, db.Close())
}

func randomIPv4Address(r *rand.Rand, ip []byte) netip.Addr {
	num := r.Uint32()
	ip[0] = byte(num >> 24)
	ip[1] = byte(num >> 16)
	ip[2] = byte(num >> 8)
	ip[3] = byte(num)
	v, _ := netip.AddrFromSlice(ip)
	return v
}

func testFile(file string) string {
	return filepath.Join("testdata", file)
}

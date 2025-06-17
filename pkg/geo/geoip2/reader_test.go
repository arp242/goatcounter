package geoip2

import (
	"math/rand"
	"net"
	"testing"
)

func TestReader(t *testing.T) {
	reader, err := Open("testdata/GeoIP2-City-Test.mmdb")
	if err != nil {
		t.Fatal(err)
	}

	defer reader.Close()

	record, err := reader.City(net.ParseIP("81.2.69.160"))
	if err != nil {
		t.Fatal(err)
	}

	m := reader.Metadata()
	equal(t, uint(2), m.BinaryFormatMajorVersion)
	equal(t, uint(0), m.BinaryFormatMinorVersion)
	notZero(t, m.BuildEpoch)
	equal(t, "GeoIP2-City", m.DatabaseType)
	equal(t, map[string]string{
		"en": "GeoIP2 City Test Database (fake GeoIP2 data, for example purposes only)",
		"zh": "小型数据库",
	}, m.Description)
	equal(t, uint(6), m.IPVersion)
	equal(t, []string{"en", "zh"}, m.Languages)
	notZero(t, m.NodeCount)
	equal(t, uint(28), m.RecordSize)

	equal(t, uint(2643743), record.City.GeoNameID)
	equal(t, map[string]string{
		"de":    "London",
		"en":    "London",
		"es":    "Londres",
		"fr":    "Londres",
		"ja":    "ロンドン",
		"pt-BR": "Londres",
		"ru":    "Лондон",
	}, record.City.Names)
	equal(t, uint(6255148), record.Continent.GeoNameID)
	equal(t, "EU", record.Continent.Code)
	equal(t, map[string]string{
		"de":    "Europa",
		"en":    "Europe",
		"es":    "Europa",
		"fr":    "Europe",
		"ja":    "ヨーロッパ",
		"pt-BR": "Europa",
		"ru":    "Европа",
		"zh-CN": "欧洲",
	}, record.Continent.Names)

	equal(t, uint(2635167), record.Country.GeoNameID)
	isFalse(t, record.Country.IsInEuropeanUnion)
	equal(t, "GB", record.Country.IsoCode)
	equal(t, map[string]string{
		"de":    "Vereinigtes Königreich",
		"en":    "United Kingdom",
		"es":    "Reino Unido",
		"fr":    "Royaume-Uni",
		"ja":    "イギリス",
		"pt-BR": "Reino Unido",
		"ru":    "Великобритания",
		"zh-CN": "英国",
	}, record.Country.Names)

	equal(t, uint16(100), record.Location.AccuracyRadius)
	inEpsilon(t, 51.5142, record.Location.Latitude, 1e-10)
	inEpsilon(t, -0.0931, record.Location.Longitude, 1e-10)
	equal(t, "Europe/London", record.Location.TimeZone)

	equal(t, uint(6269131), record.Subdivisions[0].GeoNameID)
	equal(t, "ENG", record.Subdivisions[0].IsoCode)
	equal(t, map[string]string{
		"en":    "England",
		"pt-BR": "Inglaterra",
		"fr":    "Angleterre",
		"es":    "Inglaterra",
	},
		record.Subdivisions[0].Names)

	equal(t, uint(6252001), record.RegisteredCountry.GeoNameID)
	isFalse(t, record.RegisteredCountry.IsInEuropeanUnion)
	equal(t, "US", record.RegisteredCountry.IsoCode)
	equal(t, map[string]string{
		"de":    "USA",
		"en":    "United States",
		"es":    "Estados Unidos",
		"fr":    "États-Unis",
		"ja":    "アメリカ合衆国",
		"pt-BR": "Estados Unidos",
		"ru":    "США",
		"zh-CN": "美国",
	}, record.RegisteredCountry.Names)

	isFalse(t, record.RepresentedCountry.IsInEuropeanUnion)
}

func TestIsAnycast(t *testing.T) {
	for _, test := range []string{"Country", "City", "Enterprise"} {
		t.Run(test, func(t *testing.T) {
			reader, err := Open("testdata/GeoIP2-" + test + "-Test.mmdb")
			if err != nil {
				t.Fatal(err)
			}
			defer reader.Close()

			record, err := reader.City(net.ParseIP("214.1.1.0"))
			if err != nil {
				t.Fatal(err)
			}

			isTrue(t, record.Traits.IsAnycast)
		})
	}
}

func TestMetroCode(t *testing.T) {
	reader, err := Open("testdata/GeoIP2-City-Test.mmdb")
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()

	record, err := reader.City(net.ParseIP("216.160.83.56"))
	if err != nil {
		t.Fatal(err)
	}

	equal(t, uint(819), record.Location.MetroCode)
}

func TestAnonymousIP(t *testing.T) {
	reader, err := Open("testdata/GeoIP2-Anonymous-IP-Test.mmdb")
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()

	record, err := reader.AnonymousIP(net.ParseIP("1.2.0.0"))
	if err != nil {
		t.Fatal(err)
	}

	isTrue(t, record.IsAnonymous)

	isTrue(t, record.IsAnonymousVPN)
	isFalse(t, record.IsHostingProvider)
	isFalse(t, record.IsPublicProxy)
	isFalse(t, record.IsTorExitNode)
	isFalse(t, record.IsResidentialProxy)
}

func TestASN(t *testing.T) {
	reader, err := Open("testdata/GeoLite2-ASN-Test.mmdb")
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()

	record, err := reader.ASN(net.ParseIP("1.128.0.0"))
	if err != nil {
		t.Fatal(err)
	}

	equal(t, uint(1221), record.AutonomousSystemNumber)

	equal(t, "Telstra Pty Ltd", record.AutonomousSystemOrganization)
}

func TestConnectionType(t *testing.T) {
	reader, err := Open("testdata/GeoIP2-Connection-Type-Test.mmdb")
	if err != nil {
		t.Fatal(err)
	}

	defer reader.Close()

	record, err := reader.ConnectionType(net.ParseIP("1.0.1.0"))
	if err != nil {
		t.Fatal(err)
	}

	equal(t, "Cellular", record.ConnectionType)
}

func TestCountry(t *testing.T) {
	reader, err := Open("testdata/GeoIP2-Country-Test.mmdb")
	if err != nil {
		t.Fatal(err)
	}

	defer reader.Close()

	record, err := reader.Country(net.ParseIP("81.2.69.160"))
	if err != nil {
		t.Fatal(err)
	}

	isFalse(t, record.Country.IsInEuropeanUnion)
	isFalse(t, record.RegisteredCountry.IsInEuropeanUnion)
	isFalse(t, record.RepresentedCountry.IsInEuropeanUnion)
}

func TestDomain(t *testing.T) {
	reader, err := Open("testdata/GeoIP2-Domain-Test.mmdb")
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()

	record, err := reader.Domain(net.ParseIP("1.2.0.0"))
	if err != nil {
		t.Fatal(err)
	}
	equal(t, "maxmind.com", record.Domain)
}

func TestEnterprise(t *testing.T) {
	reader, err := Open("testdata/GeoIP2-Enterprise-Test.mmdb")
	if err != nil {
		t.Fatal(err)
	}

	defer reader.Close()

	record, err := reader.Enterprise(net.ParseIP("74.209.24.0"))
	if err != nil {
		t.Fatal(err)
	}

	equal(t, uint8(11), record.City.Confidence)

	equal(t, uint(14671), record.Traits.AutonomousSystemNumber)
	equal(t, "FairPoint Communications", record.Traits.AutonomousSystemOrganization)
	equal(t, "Cable/DSL", record.Traits.ConnectionType)
	equal(t, "frpt.net", record.Traits.Domain)
	inEpsilon(t, float64(0.34), record.Traits.StaticIPScore, 1e-10)

	record, err = reader.Enterprise(net.ParseIP("149.101.100.0"))
	if err != nil {
		t.Fatal(err)
	}

	equal(t, uint(6167), record.Traits.AutonomousSystemNumber)

	equal(t, "CELLCO-PART", record.Traits.AutonomousSystemOrganization)
	equal(t, "Verizon Wireless", record.Traits.ISP)
	equal(t, "310", record.Traits.MobileCountryCode)
	equal(t, "004", record.Traits.MobileNetworkCode)
}

func TestISP(t *testing.T) {
	reader, err := Open("testdata/GeoIP2-ISP-Test.mmdb")
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()

	record, err := reader.ISP(net.ParseIP("149.101.100.0"))
	if err != nil {
		t.Fatal(err)
	}

	equal(t, uint(6167), record.AutonomousSystemNumber)

	equal(t, "CELLCO-PART", record.AutonomousSystemOrganization)
	equal(t, "Verizon Wireless", record.ISP)
	equal(t, "310", record.MobileCountryCode)
	equal(t, "004", record.MobileNetworkCode)
	equal(t, "Verizon Wireless", record.Organization)
}

// This ensures the compiler does not optimize away the function call.
var cityResult *City

func BenchmarkCity(b *testing.B) {
	db, err := Open("GeoLite2-City.mmdb")
	if err != nil {
		b.Fatal(err)
	}
	defer db.Close()

	//nolint:gosec // this is just a benchmark
	r := rand.New(rand.NewSource(0))

	var city *City

	ip := make(net.IP, 4)
	for range b.N {
		randomIPv4Address(r, ip)
		city, err = db.City(ip)
		if err != nil {
			b.Fatal(err)
		}
	}
	cityResult = city
}

// This ensures the compiler does not optimize away the function call.
var asnResult *ASN

func BenchmarkASN(b *testing.B) {
	db, err := Open("GeoLite2-ASN.mmdb")
	if err != nil {
		b.Fatal(err)
	}
	defer db.Close()

	//nolint:gosec // this is just a benchmark
	r := rand.New(rand.NewSource(0))

	var asn *ASN

	ip := make(net.IP, 4)
	for range b.N {
		randomIPv4Address(r, ip)
		asn, err = db.ASN(ip)
		if err != nil {
			b.Fatal(err)
		}
	}
	asnResult = asn
}

func randomIPv4Address(r *rand.Rand, ip net.IP) {
	num := r.Uint32()
	ip[0] = byte(num >> 24)
	ip[1] = byte(num >> 16)
	ip[2] = byte(num >> 8)
	ip[3] = byte(num)
}

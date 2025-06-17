package maxminddb_test

import (
	"fmt"
	"log"
	"net/netip"

	"zgo.at/goatcounter/v2/pkg/geo/maxminddb"
)

// This example shows how to decode to a struct.
func ExampleReader_Lookup_struct() {
	db, err := maxminddb.Open("testdata/GeoIP2-City-Test.mmdb")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	addr := netip.MustParseAddr("81.2.69.142")

	var record struct {
		Country struct {
			ISOCode string `maxminddb:"iso_code"`
		} `maxminddb:"country"`
	} // Or any appropriate struct

	err = db.Lookup(addr).Decode(&record)
	if err != nil {
		log.Panic(err)
	}
	fmt.Print(record.Country.ISOCode)
	// Output:
	// GB
}

// This example demonstrates how to decode to an any.
func ExampleReader_Lookup_interface() {
	db, err := maxminddb.Open("testdata/GeoIP2-City-Test.mmdb")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	addr := netip.MustParseAddr("81.2.69.142")

	var record any
	err = db.Lookup(addr).Decode(&record)
	if err != nil {
		log.Panic(err)
	}
	fmt.Printf("%v", record)
	//nolint:lll
	// Output:
	// map[city:map[geoname_id:2643743 names:map[de:London en:London es:Londres fr:Londres ja:ロンドン pt-BR:Londres ru:Лондон]] continent:map[code:EU geoname_id:6255148 names:map[de:Europa en:Europe es:Europa fr:Europe ja:ヨーロッパ pt-BR:Europa ru:Европа zh-CN:欧洲]] country:map[geoname_id:2635167 iso_code:GB names:map[de:Vereinigtes Königreich en:United Kingdom es:Reino Unido fr:Royaume-Uni ja:イギリス pt-BR:Reino Unido ru:Великобритания zh-CN:英国]] location:map[accuracy_radius:10 latitude:51.5142 longitude:-0.0931 time_zone:Europe/London] registered_country:map[geoname_id:6252001 iso_code:US names:map[de:USA en:United States es:Estados Unidos fr:États-Unis ja:アメリカ合衆国 pt-BR:Estados Unidos ru:США zh-CN:美国]] subdivisions:[map[geoname_id:6269131 iso_code:ENG names:map[en:England es:Inglaterra fr:Angleterre pt-BR:Inglaterra]]]]
}

// This example demonstrates how to iterate over all networks in the
// database.
func ExampleReader_Networks() {
	db, err := maxminddb.Open("testdata/GeoIP2-Connection-Type-Test.mmdb")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	for result := range db.Networks() {
		record := struct {
			Domain string `maxminddb:"connection_type"`
		}{}

		err := result.Decode(&record)
		if err != nil {
			log.Panic(err)
		}
		fmt.Printf("%s: %s\n", result.Prefix(), record.Domain)
	}
	// Output:
	// 1.0.0.0/24: Cable/DSL
	// 1.0.1.0/24: Cellular
	// 1.0.2.0/23: Cable/DSL
	// 1.0.4.0/22: Cable/DSL
	// 1.0.8.0/21: Cable/DSL
	// 1.0.16.0/20: Cable/DSL
	// 1.0.32.0/19: Cable/DSL
	// 1.0.64.0/18: Cable/DSL
	// 1.0.128.0/17: Cable/DSL
	// 2.125.160.216/29: Cable/DSL
	// 67.43.156.0/24: Cellular
	// 80.214.0.0/20: Cellular
	// 96.1.0.0/16: Cable/DSL
	// 96.10.0.0/15: Cable/DSL
	// 96.69.0.0/16: Cable/DSL
	// 96.94.0.0/15: Cable/DSL
	// 108.96.0.0/11: Cellular
	// 149.101.100.0/28: Cellular
	// 175.16.199.0/24: Cable/DSL
	// 187.156.138.0/24: Cable/DSL
	// 201.243.200.0/24: Corporate
	// 207.179.48.0/20: Cellular
	// 216.160.83.56/29: Corporate
	// 2003::/24: Cable/DSL
}

// This example demonstrates how to iterate over all networks in the
// database which are contained within an arbitrary network.
func ExampleReader_NetworksWithin() {
	db, err := maxminddb.Open("testdata/GeoIP2-Connection-Type-Test.mmdb")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	prefix, err := netip.ParsePrefix("1.0.0.0/8")
	if err != nil {
		log.Panic(err)
	}

	for result := range db.NetworksWithin(prefix) {
		record := struct {
			Domain string `maxminddb:"connection_type"`
		}{}
		err := result.Decode(&record)
		if err != nil {
			log.Panic(err)
		}
		fmt.Printf("%s: %s\n", result.Prefix(), record.Domain)
	}

	// Output:
	// 1.0.0.0/24: Cable/DSL
	// 1.0.1.0/24: Cellular
	// 1.0.2.0/23: Cable/DSL
	// 1.0.4.0/22: Cable/DSL
	// 1.0.8.0/21: Cable/DSL
	// 1.0.16.0/20: Cable/DSL
	// 1.0.32.0/19: Cable/DSL
	// 1.0.64.0/18: Cable/DSL
	// 1.0.128.0/17: Cable/DSL
}

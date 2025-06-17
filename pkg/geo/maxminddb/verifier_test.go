package maxminddb

import (
	"testing"
)

func TestVerifyOnGoodDatabases(t *testing.T) {
	databases := []string{
		"GeoIP2-Anonymous-IP-Test.mmdb",
		"GeoIP2-City-Test.mmdb",
		"GeoIP2-Connection-Type-Test.mmdb",
		"GeoIP2-Country-Test.mmdb",
		"GeoIP2-Domain-Test.mmdb",
		"GeoIP2-ISP-Test.mmdb",
		"GeoIP2-Precision-Enterprise-Test.mmdb",
		"MaxMind-DB-no-ipv4-search-tree.mmdb",
		"MaxMind-DB-string-value-entries.mmdb",
		"MaxMind-DB-test-decoder.mmdb",
		"MaxMind-DB-test-ipv4-24.mmdb",
		"MaxMind-DB-test-ipv4-28.mmdb",
		"MaxMind-DB-test-ipv4-32.mmdb",
		"MaxMind-DB-test-ipv6-24.mmdb",
		"MaxMind-DB-test-ipv6-28.mmdb",
		"MaxMind-DB-test-ipv6-32.mmdb",
		"MaxMind-DB-test-mixed-24.mmdb",
		"MaxMind-DB-test-mixed-28.mmdb",
		"MaxMind-DB-test-mixed-32.mmdb",
		"MaxMind-DB-test-nested.mmdb",
	}

	for _, database := range databases {
		t.Run(database, func(t *testing.T) {
			reader, err := Open(testFile(database))
			if err != nil {
				t.Fatal(err)
			}

			err = reader.Verify()
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestVerifyOnBrokenDatabases(t *testing.T) {
	databases := []string{
		"GeoIP2-City-Test-Broken-Double-Format.mmdb",
		"MaxMind-DB-test-broken-pointers-24.mmdb",
		"MaxMind-DB-test-broken-search-tree-24.mmdb",
	}

	for _, database := range databases {
		reader, err := Open(testFile(database))
		if err != nil {
			t.Fatal(err)
		}
		err = reader.Verify()
		if err == nil {
			t.Fatal("Verify() err is nil")
		}
	}
}

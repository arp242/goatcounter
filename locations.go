// Copyright © 2019 Martin Tournoij – This file is part of GoatCounter and
// published under the terms of a slightly modified EUPL v1.2 license, which can
// be found in the LICENSE file or at https://license.goatcounter.com

package goatcounter

import (
	"bytes"
	"compress/gzip"
	"context"
	"io"
	"net"

	"github.com/oschwald/geoip2-golang"
	"zgo.at/errors"
	"zgo.at/zdb"
	"zgo.at/zlog"
	"zgo.at/zstd/zstring"
)

var geodb *geoip2.Reader

// InitGeoDB sets up the geoDB database located at the given path.
//
// The database can be the "Countries" or "Cities" version.
//
// It will use the embeded "Countries" database if path is an empty string.
func InitGeoDB(path string) {
	var err error

	if path != "" {
		geodb, err = geoip2.Open(path)
		if err != nil {
			panic(err)
		}
		GeoDB = nil // Save some memory.
		return
	}

	gz, err := gzip.NewReader(bytes.NewReader(GeoDB))
	if err != nil {
		panic(err)
	}
	d, err := io.ReadAll(gz)
	if err != nil {
		panic(err)
	}
	geodb, err = geoip2.FromBytes(d)
	if err != nil {
		panic(err)
	}
}

type Location struct {
	ID int64 `db:"location_id"`

	Country string `db:"country"`
	Region  string `db:"region"`

	// TODO(i18n): the geoDB actually contains translated country names, but we
	// don't store them. We can run the country name through z18n, i.e.
	//
	//   create table locations_i18n (
	//     locale       text,  -- nl_NL
	//     messages     text,  [{"Netherlands": "Nederland", ..}]
	//   );

	CountryName string `db:"country_name"`
	RegionName  string `db:"region_name"`

	// TODO: send patch to staticcheck to deal with this better. This shouldn't
	// errror since "ISO" is an initialism.
	ISO3166_2 string `db:"iso_3166_2"` //lint:ignore ST1003 staticcheck bug
}

// ByCode gets a location by ISO-3166-2 code; e.g. "US" or "US-TX".
func (l *Location) ByCode(ctx context.Context, code string) error {
	if ll, ok := cacheLoc(ctx).Get(code); ok {
		*l = *ll.(*Location)
		return nil
	}

	err := zdb.Get(ctx, l, `select * from locations where iso_3166_2 = $1`, code)
	if zdb.ErrNoRows(err) {
		l.ISO3166_2 = code
		l.Country, l.Region = zstring.Split2(code, "-")
		l.CountryName, l.RegionName = findGeoName(l.Country, l.Region)
		err = l.insert(ctx)
	}
	if err != nil {
		return errors.Wrap(err, "Location.ByCode")
	}

	cacheLoc(ctx).SetDefault(l.ISO3166_2, l)
	return nil
}

// Lookup a location by IPv4 or IPv6 address.
//
// This will insert a row in the locations table if one doesn't exist yet.
func (l *Location) Lookup(ctx context.Context, ip string) error {
	if geodb == nil {
		panic("Location.Lookup: geo.Init not called")
	}

	loc, err := geodb.City(net.ParseIP(ip))
	if err != nil {
		return errors.Wrap(err, "Location.Lookup")
	}
	l.Country = loc.Country.IsoCode
	l.CountryName = loc.Country.Names["en"]
	if len(loc.Subdivisions) > 0 {
		l.Region, l.RegionName = loc.Subdivisions[0].IsoCode, loc.Subdivisions[0].Names["en"]
	}

	l.ISO3166_2 = loc.Country.IsoCode
	if l.Region != "" {
		l.ISO3166_2 += "-" + l.Region
	}
	if ll, ok := cacheLoc(ctx).Get(l.ISO3166_2); ok {
		*l = *ll.(*Location)
		return nil
	}

	err = zdb.Get(ctx, l,
		`select * from locations where country = $1 and region = $2`,
		l.Country, l.Region)
	if zdb.ErrNoRows(err) {
		err = l.insert(ctx)
	}
	if err != nil {
		return errors.Wrap(err, "Location.Lookup")
	}

	cacheLoc(ctx).SetDefault(l.ISO3166_2, l)
	return nil
}

// LookupIP is a shorthand for Lookup(); returns id 1 on errors ("unknown").
func (l Location) LookupIP(ctx context.Context, ip string) string {
	err := l.Lookup(ctx, ip)
	if err != nil {
		return "" // Special ID: "unknown".
	}
	return l.ISO3166_2
}

func (l *Location) insert(ctx context.Context) (err error) {
	l.ID, err = zdb.InsertID(ctx, "location_id",
		`insert into locations (country, region, country_name, region_name) values (?, ?, ?, ?)`,
		l.Country, l.Region, l.CountryName, l.RegionName)
	if err != nil {
		return err
	}

	// Make sure there is an entry for the country as well.
	if l.Region != "" {
		err := (&Location{}).ByCode(ctx, l.Country)
		if err != nil {
			return err
		}
	}
	return nil
}

type Locations []Location

// ListCountries lists all counties. The region code/name will always be blank.
func (l *Locations) ListCountries(ctx context.Context) error {
	err := zdb.Select(ctx, l, `
		select country, country_name
        from locations
        where country != '' and country_name != '' and region = ''
        order by country_name`)
	return errors.Wrap(err, "Locations.ListCountries")
}

// This takes ~13s for a full iteration for the Cities database on my laptop
// (Countries is much faster, ~100ms) which is not a great worst case scenario,
// but in most cases it should be (much) faster, and this should get called
// extremely infrequently anyway, if ever.
func findGeoName(country, region string) (string, string) {
	hasRegions := geodb.Metadata().DatabaseType == "City"
	iter := geodb.DB().Data()
	for iter.Next() {
		var r struct {
			Country struct {
				ISOCode string            `maxminddb:"iso_code"`
				Names   map[string]string `maxminddb:"names"`
			} `maxminddb:"country"`
			Subdivisions []struct {
				ISOCode string            `maxminddb:"iso_code"`
				Names   map[string]string `maxminddb:"names"`
			} `maxminddb:"subdivisions"`
		}
		err := iter.Data(&r)
		if err != nil {
			zlog.Error(err)
			return "", ""
		}

		switch {
		// Country database, no region.
		case r.Country.ISOCode == country && !hasRegions:
			return r.Country.Names["en"], ""
		// City database, no region requested.
		case r.Country.ISOCode == country && region == "":
			return r.Country.Names["en"], ""
		// Match region.
		case r.Country.ISOCode == country && len(r.Subdivisions) > 0 && r.Subdivisions[0].ISOCode == region:
			return r.Country.Names["en"], r.Subdivisions[0].Names["en"]
		}
	}
	return "", ""
}

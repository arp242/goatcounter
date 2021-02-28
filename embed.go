// Copyright © 2019 Martin Tournoij – This file is part of GoatCounter and
// published under the terms of a slightly modified EUPL v1.2 license, which can
// be found in the LICENSE file or at https://license.goatcounter.com

package goatcounter

import "embed"

// DB contains all files in db/*
//
//go:embed db/schema-postgres.sql
//go:embed db/schema-sqlite.sql
//go:embed db/migrate/*.sql
//go:embed db/query/*.sql
var DB embed.FS

// Static contains all the static files to serve.
//go:embed public/*
var Static embed.FS

// Templates contains all templates.
//go:embed tpl/*
var Templates embed.FS

// GeoDB contains the GeoIP countries database.
//
//go:embed pack/GeoLite2-Country.mmdb.gz
var GeoDB []byte

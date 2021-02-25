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
var DB embed.FS

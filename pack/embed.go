// Copyright © 2019 Martin Tournoij – This file is part of GoatCounter and
// published under the terms of a slightly modified EUPL v1.2 license, which can
// be found in the LICENSE file or at https://license.goatcounter.com

package pack

import _ "embed"

// GeoDB contains the GeoIP countries database.
//
//go:embed GeoLite2-Country.mmdb.gz
var GeoDB []byte

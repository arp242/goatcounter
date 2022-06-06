// Copyright © Martin Tournoij – This file is part of GoatCounter and published
// under the terms of a slightly modified EUPL v1.2 license, which can be found
// in the LICENSE file or at https://license.goatcounter.com

//go:build !go1.18
// +build !go1.18

package main

// Make sure people don't try to build GoatCounter with older versions of Go, as
// that will introduce some runtime problems (e.g. using %w).
func init() {
	"You need Go 1.18 or newer to compile GoatCounter"
}

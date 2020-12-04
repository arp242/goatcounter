// +build !go1.13

package main

// Make sure people don't try to build GoatCounter with older versions of Go, as
// that will introduce some runtime problems (e.g. using %w).
func init() {
	"You need Go 1.13 or newer to compile GoatCounter"
}

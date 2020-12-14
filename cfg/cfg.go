// Copyright © 2019 Martin Tournoij – This file is part of GoatCounter and
// published under the terms of a slightly modified EUPL v1.2 license, which can
// be found in the LICENSE file or at https://license.goatcounter.com

// Package cfg contains global application configuration settings.
package cfg

// Configuration variables.
//
// TODO: much of this realy shouldn't be in a global package like this.
var (
	Domain         string
	DomainStatic   string
	DomainCount    string
	URLStatic      string
	Plan           string
	Prod           bool
	Version        string
	GoatcounterCom bool
	Serve          bool
	Port           string
	EmailFrom      string

	PgSQL bool // TODO: replace with zdb.PgSQL

	RunningTests bool
)

func Reset() {
	Domain = ""
	DomainStatic = ""
	DomainCount = ""
	URLStatic = ""
	PgSQL = false
	Plan = ""
	Prod = false
	Version = ""
	GoatcounterCom = false
	Serve = false
	Port = ""
	EmailFrom = ""
	RunningTests = false
}

// Copyright © 2019 Martin Tournoij <martin@arp242.net>
// This file is part of GoatCounter and published under the terms of the EUPL
// v1.2, which can be found in the LICENSE file or at http://eupl12.zgo.at

// Package cfg contains global application configuration settings.
package cfg

// Configuration variables.
var (
	Domain         string
	DomainStatic   string
	DomainCount    string
	URLStatic      string
	PgSQL          bool
	Plan           string
	Prod           bool
	Version        string
	GoatcounterCom bool
	Serve          bool
	Port           string
	EmailFrom      string
)

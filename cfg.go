// Copyright © 2019 Martin Tournoij – This file is part of GoatCounter and
// published under the terms of a slightly modified EUPL v1.2 license, which can
// be found in the LICENSE file or at https://license.goatcounter.com

package goatcounter

import "context"

var Version = ""

type GlobalConfig struct {
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
	RunningTests   bool
}

var keyConfig = &struct{ n string }{""}

func NewConfig(ctx context.Context) context.Context {
	return context.WithValue(ctx, keyConfig, &GlobalConfig{})
}

func Config(ctx context.Context) *GlobalConfig {
	return ctx.Value(keyConfig).(*GlobalConfig)
}

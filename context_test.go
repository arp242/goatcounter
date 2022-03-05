// Copyright © Martin Tournoij – This file is part of GoatCounter and published
// under the terms of a slightly modified EUPL v1.2 license, which can be found
// in the LICENSE file or at https://license.goatcounter.com

package goatcounter

import (
	"context"
	"testing"
)

func TestContext(t *testing.T) {
	ctx := context.Background()
	{
		c := cacheSites(ctx)
		if c == nil {
			t.Error("c is nil")
		}

		cfg := Config(ctx)
		if cfg == nil {
			t.Error("cfg is nil")
		}
	}

	ctx = NewCache(ctx)
	{
		c1 := cacheSites(ctx)
		c2 := cacheSites(ctx)
		if c1 != c2 {
			t.Errorf("%v %v", c1, c2)
		}
	}

	ctx = NewConfig(ctx)
	{
		c1 := Config(ctx)
		c2 := Config(ctx)
		if c1 != c2 {
			t.Errorf("%v %v", c1, c2)
		}
	}
}

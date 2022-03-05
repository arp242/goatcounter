// Copyright © Martin Tournoij – This file is part of GoatCounter and published
// under the terms of a slightly modified EUPL v1.2 license, which can be found
// in the LICENSE file or at https://license.goatcounter.com

package bgrun

import (
	"context"
	"testing"
	"time"

	"zgo.at/errors"
)

func TestRun(t *testing.T) {
	i := 0
	Run("test 1", func() {
		time.Sleep(200 * time.Millisecond)
		i = 1
	})
	err := Wait(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if i != 1 {
		t.Fatal("i not set")
	}
}

func TestWait(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	Run("test wait", func() { time.Sleep(2 * time.Second) })
	err := Wait(ctx)
	if err == nil {
		t.Fatal("error is nil")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("wrong error; %#v", err)
	}

	time.Sleep(2 * time.Second)
}

// Copyright © 2019 Martin Tournoij <martin@arp242.net>
// This file is part of GoatCounter and published under the terms of the EUPL
// v1.2, which can be found in the LICENSE file or at http://eupl12.zgo.at

package goatcounter_test

import (
	"sync"
	"testing"
	"time"

	"zgo.at/goatcounter"
	"zgo.at/goatcounter/gctest"
)

func TestSessionSalt(t *testing.T) {
	// Depends on timings and is kinda hard to test :-/
	t.Skip()

	ctx, clean := gctest.DB(t)
	defer clean()

	goatcounter.Salts = goatcounter.Salt{
		CycleEvery: 1 * time.Second,
	}

	cur1, prev1 := goatcounter.Salts.Get(ctx)
	if cur1 == "" || prev1 == "" {
		t.Fatal("empty?")
	}

	time.Sleep(100 * time.Millisecond)
	err := goatcounter.Salts.Refresh(ctx)
	if err != nil {
		t.Fatal(err)
	}
	cur2, prev2 := goatcounter.Salts.Get(ctx)
	if cur1 != cur2 || prev1 != prev2 {
		t.Fatalf("not identical\ncur:  %s != %s\nprev: %s != %s", cur1, cur2, prev1, prev2)
	}

	time.Sleep(800 * time.Millisecond)
	err = goatcounter.Salts.Refresh(ctx)
	if err != nil {
		t.Fatal(err)
	}

	cur3, prev3 := goatcounter.Salts.Get(ctx)
	if cur2 == cur3 || prev2 == prev3 {
		t.Fatalf("not cycled\ncur:  %s == %s\nprev: %s == %s", cur2, cur3, prev2, prev3)
	}

	goatcounter.Salts = goatcounter.Salt{
		CycleEvery: 1 * time.Second,
	}

	cur4, prev4 := goatcounter.Salts.Get(ctx)
	if cur3 != cur4 || prev3 != prev4 {
		t.Fatalf("not persisted\ncur:  %s != %s\nprev: %s != %s", cur3, cur4, prev3, prev4)
	}
}

func TestSessionGetOrCreate(t *testing.T) {
	t.Skip() // TODO: difficult to test concurency with SQLite :memory: :-/

	ctx, clean := gctest.DB(t)
	defer clean()

	type test struct {
		session              *goatcounter.Session
		path, ua, remoteAddr string

		created bool
		err     error
	}

	const n = 10
	var data [n]test
	for i := 0; i < n; i++ {
		data[i] = test{session: &goatcounter.Session{}, path: "/test", ua: "test", remoteAddr: "127.0.0.1"}
	}

	var wg sync.WaitGroup
	wg.Add(n - 1)
	data[0].created, data[0].err = data[0].session.GetOrCreate(ctx, data[0].path, data[0].ua, data[0].remoteAddr)
	for i := 1; i < n; i++ {
		go func(i int) {
			data[i].created, data[i].err = data[i].session.GetOrCreate(ctx, data[i].path, data[i].ua, data[i].remoteAddr)
			wg.Done()
		}(i)
	}
	wg.Wait()

	for i, d := range data {
		if d.err != nil {
			t.Error(d.err)
		}
		if i == 0 && !d.created {
			t.Error("first not created")
		}

		if i > 0 && d.created {
			t.Error(">0 created")
		}
	}
}

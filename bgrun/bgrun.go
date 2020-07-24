// Copyright © 2019 Martin Tournoij – This file is part of GoatCounter and
// published under the terms of a slightly modified EUPL v1.2 license, which can
// be found in the LICENSE file or at https://license.goatcounter.com

// Package bgrun allows simple synchronisation of goroutines.
//
// This is mostly intended for "fire and forget" type of goroutines like sending
// an email. They typically don't really need any synchronisation as such but
// you do want to wait for them to finish before the program exits, or you want
// to wait for them in tests.
package bgrun

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"zgo.at/errors"
	"zgo.at/zli"
	"zgo.at/zlog"
	"zgo.at/zstd/zsync"
)

var (
	wg      = new(sync.WaitGroup)
	maxWait = 2 * time.Minute

	working struct {
		sync.Mutex
		m map[string]struct{}
	}
)

// Wait for all goroutines to finish for a maximum of maxWait.
func Wait() error {
	ctx, c := context.WithTimeout(context.Background(), maxWait)
	defer c()

	return errors.Wrap(zsync.Wait(ctx, wg), "bgrun.Wait")
}

// WaitProgress calls Wait() and prints which tasks it's waiting for.
func WaitProgress() error {
	term := zli.IsTerminal(os.Stdout.Fd())

	go func() {
		working.Lock()
		if len(working.m) == 0 {
			return
		}
		working.Unlock()

		for {
			if term {
				zli.EraseLine(2)
			}

			working.Lock()
			fmt.Printf("\r%d tasks: ", len(working.m))
			l := make([]string, 0, len(working.m))
			for k := range working.m {
				l = append(l, k)
			}
			sort.Strings(l)
			if term {
				fmt.Print(strings.Join(l, ", "), " ")
			}
			working.Unlock()

			time.Sleep(200 * time.Millisecond)
			working.Lock()
			if len(working.m) == 0 {
				return
			}
			working.Unlock()
		}
	}()

	err := Wait()
	if term {
		zli.EraseLine(2)
		fmt.Print("\r done \n")
	}
	return err
}

// WaitAndLog calls Wait() and logs any errors.
func WaitAndLog() {
	err := Wait()
	if err != nil {
		zlog.Error(err)
	}
}

// WaitProgressAndLog calls Wait(), prints which tasks it's waiting for, and
// logs any errors.
func WaitProgressAndLog() {
	err := WaitProgress()
	if err != nil {
		zlog.Error(err)
	}
}

// Run the function in a goroutine.
//
//   bgrun.Run(func() {
//       // Do work...
//   })
func Run(name string, f func()) {
	done := Add(name)
	go func() {
		defer zlog.Recover()
		defer done()
		f()
	}()
}

// Add a new function to the waitgroup and return the done.
//
//    done := bgrun.Add()
//    go func() {
//       defer done()
//       defer zlog.Recover()
//    }()
func Add(name string) func() {
	wg.Add(1)
	working.Lock()
	if working.m == nil {
		working.m = make(map[string]struct{})
	}
	working.m[name] = struct{}{}
	working.Unlock()

	return func() {
		wg.Done()
		working.Lock()
		delete(working.m, name)
		working.Unlock()
	}
}

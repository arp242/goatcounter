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

	// TODO: this won't actually kill the goroutines that are still running.
	return errors.Wrap(zsync.Wait(ctx, wg), "bgrun.Wait")
}

// WaitProgress calls Wait() and prints which tasks it's waiting for.
func WaitProgress() error {
	term := zli.IsTerminal(os.Stdout.Fd())

	go func() {
		func() {
			working.Lock()
			defer working.Unlock()
			if len(working.m) == 0 {
				return
			}
		}()

		for {
			if term {
				zli.EraseLine()
			}

			func() {
				working.Lock()
				defer working.Unlock()
				if len(working.m) == 0 {
					if term {
						fmt.Println()
					}
					return
				}

				if term {
					fmt.Printf("%d tasks: ", len(working.m))
				}
				l := make([]string, 0, len(working.m))
				for k := range working.m {
					l = append(l, k)
				}
				sort.Strings(l)
				if term {
					fmt.Print(strings.Join(l, ", "), " ")
				}
			}()

			time.Sleep(100 * time.Millisecond)
			func() {
				working.Lock()
				defer working.Unlock()
				if len(working.m) == 0 {
					if term {
						fmt.Println()
					}
					return
				}
			}()
		}
	}()

	err := Wait()
	if term {
		zli.EraseLine()
		fmt.Print(" done \n")
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

// RunNoDuplicates is like Run(), but only allows one instance of this name.
//
// It will do nothing if there's already something running with this name.
func RunNoDuplicates(name string, f func()) {
	if Running(name) {
		return
	}
	Run(name, f)
}

// Running reports if a function by this name is already running.
func Running(name string) bool {
	working.Lock()
	defer working.Unlock()
	_, ok := working.m[name]
	return ok
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
	func() {
		working.Lock()
		defer working.Unlock()
		if working.m == nil {
			working.m = make(map[string]struct{})
		}
		working.m[name] = struct{}{}
	}()

	return func() {
		wg.Done()
		func() {
			working.Lock()
			defer working.Unlock()
			delete(working.m, name)
		}()
	}
}

// Copyright © Martin Tournoij – This file is part of GoatCounter and published
// under the terms of a slightly modified EUPL v1.2 license, which can be found
// in the LICENSE file or at https://license.goatcounter.com

package deferr

import "fmt"

func x() {
	// Correct.
	defer f1()
	defer func() { fmt.Println("X") }()
	defer f2()()
	defer func() func() { return func() {} }()()

	ch := make(chan struct{})
	defer close(ch)
	var a, b []string
	defer copy(a, b)

	// Wrong.
	defer f2()                                 // want "defered return not called"
	defer func() func() { return func() {} }() // want "defered return not called"
	defer f3()                                 // want "defered return not called"

	// Return return function returns a function. This is getting silly and is
	// not checked.
	defer silly1()()
	defer func() func() func() {
		return func() func() {
			return func() {}
		}
	}()()
}

func f1()                       {}
func f2() func()                { return func() {} }
func f3() (string, int, func()) { return "", 0, func() {} }

func silly1() func() func() {
	return func() func() {
		return func() {}
	}
}

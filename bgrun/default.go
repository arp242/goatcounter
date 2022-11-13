package bgrun

import (
	"context"
	"fmt"
	"io"
	"os"
	"time"
)

// TODO: probably want to get rid of this; just easier for now to migrate things
// from the old bgrun to this.

var stderr io.Writer = os.Stderr

var defaultRunner = func() *Runner {
	r := NewRunner(func(t string, err error) {
		fmt.Fprintf(stderr, "bgrun: error running task %q: %s\n", t, err.Error())
	})
	r.depth = 3
	return r
}()

func NewTask(name string, maxPar int, f func(context.Context) error) {
	defaultRunner.NewTask(name, maxPar, f)
}
func Reset()                                                 { defaultRunner.Reset() }
func Run(name string, fun func(context.Context) error) error { return defaultRunner.Run(name, fun) }
func MustRun(name string, fun func(context.Context) error)   { defaultRunner.MustRun(name, fun) }
func RunFunction(name string, fun func()) error              { return defaultRunner.RunFunction(name, fun) }
func MustRunFunction(name string, fun func())                { defaultRunner.MustRunFunction(name, fun) }
func MustRunTask(name string)                                { defaultRunner.MustRunTask(name) }
func RunTask(name string) error                              { return defaultRunner.RunTask(name) }
func Wait(name string)                                       { defaultRunner.Wait(name) }
func WaitFor(d time.Duration, name string) error             { return defaultRunner.WaitFor(d, name) }
func History(newSize int) []Job                              { return defaultRunner.History(newSize) }
func Running() []Job                                         { return defaultRunner.Running() }

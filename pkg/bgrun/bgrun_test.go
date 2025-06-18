package bgrun

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"reflect"
	"sort"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

var (
	i        atomic.Int32
	testFunc = func(context.Context) error { time.Sleep(50 * time.Millisecond); i.Add(1); return nil }
)

func init() {
	NewTask("test", 1, testFunc)
}

func reset() {
	Reset()
	i = atomic.Int32{}
	NewTask("test", 1, testFunc)
}

func TestRun(t *testing.T) {
	defer reset()

	MustRunTask("test")
	Wait("test")
	if i.Load() != 1 {
		t.Fatalf("i is %d, not 1", i.Load())
	}

	MustRunTask("test")
	Wait("test")
	if i.Load() != 2 {
		t.Fatalf("i is %d, not 2", i.Load())
	}
}

func TestMultiple(t *testing.T) {
	defer reset()

	MustRunTask("test")
	MustRunTask("test")
	Wait("test")
	if i.Load() != 2 {
		t.Fatalf("i is %d, not 2", i.Load())
	}
}

func TestWaitAll(t *testing.T) {
	defer reset()

	NewTask("test2", 2, testFunc)

	MustRunTask("test")
	MustRunTask("test")
	MustRunTask("test2")
	MustRunTask("test2")
	Wait("")
	if i.Load() != 4 {
		t.Fatalf("i is %d, not 4", i.Load())
	}
}

func TestWaitFor(t *testing.T) {
	defer reset()

	NewTask("test2", 1, func(ctx context.Context) error {
		select {
		case <-ctx.Done():
		case <-time.After(50 * time.Millisecond):
			t.Error("not cancelled")
		}
		return nil
	})
	MustRunTask("test2")
	MustRunTask("test")

	err := WaitFor(2*time.Millisecond, "")
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Error(err)
	}

	err = WaitFor(50*time.Millisecond, "")
	if err != nil {
		t.Error(err)
	}
}

func jobList(h []Job) []string {
	s := make([]string, 0, len(h))
	for _, hh := range h {
		s = append(s, fmt.Sprintf("%s %s %s %s", hh.Task,
			hh.Started.Format("2006-01-02"),
			hh.Took.Truncate(time.Millisecond*10),
			hh.From[:strings.Index(hh.From, ":")]))
	}
	sort.Strings(s)
	return s
}

func TestHistory(t *testing.T) {
	t.Skip() // TODO: skipped because it doesn't mock out the time
	defer reset()

	NewTask("test2", 2, testFunc)
	NewTask("test3", 2, testFunc)
	NewTask("test4", 2, testFunc)

	MustRunTask("test")
	MustRunTask("test")
	MustRunTask("test2")
	MustRunTask("test2")
	MustRunTask("test3")
	MustRunTask("test3")
	Wait("")

	want := []string{
		"test 2022-11-13 50ms bgrun_test.go",
		"test 2022-11-13 50ms bgrun_test.go",
		"test2 2022-11-13 50ms bgrun_test.go",
		"test2 2022-11-13 50ms bgrun_test.go",
		"test3 2022-11-13 50ms bgrun_test.go",
		"test3 2022-11-13 50ms bgrun_test.go",
	}
	if h := jobList(History(0)); !reflect.DeepEqual(h, want) {
		t.Errorf("\nhave: %#v\nwant: %#v", h, want)
	}

	if h := jobList(History(2)); !reflect.DeepEqual(h, want) {
		t.Errorf("\nhave: %#v\nwant: %#v", h, want)
	}
	if l := len(History(0)); l != 2 {
		t.Error(l)
	}

	if l := len(History(-1)); l != 2 {
		t.Error(l)
	}
	if l := len(History(0)); l != 0 {
		t.Error(l)
	}

	History(2)
	MustRunTask("test")
	if l := len(History(0)); l != 0 {
		t.Error(l)
	}
	Wait("")
	want = []string{
		"test 2022-11-13 50ms bgrun_test.go",
	}
	if h := jobList(History(0)); !reflect.DeepEqual(h, want) {
		t.Errorf("\nhave: %#v\nwant: %#v", h, want)
	}

	MustRunTask("test")
	MustRunTask("test")
	MustRunTask("test")
	Wait("")
	want = []string{
		"test 2022-11-13 50ms bgrun_test.go",
		"test 2022-11-13 50ms bgrun_test.go",
	}
	if h := jobList(History(0)); !reflect.DeepEqual(h, want) {
		t.Errorf("\nhave: %#v\nwant: %#v", h, want)
	}
}

func TestRunning(t *testing.T) {
	t.Skip() // TODO: skipped because it doesn't mock out the time
	defer reset()

	NewTask("test2", 2, testFunc)
	MustRunTask("test")
	MustRunTask("test2")
	RunTask("test")

	want := []string{
		"test 2022-11-13 0s bgrun_test.go",
		"test 2022-11-13 0s bgrun_test.go",
		"test2 2022-11-13 0s bgrun_test.go",
	}
	if h := jobList(Running()); !reflect.DeepEqual(h, want) {
		t.Errorf("\nhave: %#v\nwant: %#v", h, want)
	}
	Wait("")

	if h := jobList(Running()); !reflect.DeepEqual(h, []string{}) {
		t.Errorf("\nhave: %#v\nwant: %#v", h, want)
	}
}

func TestLog(t *testing.T) {
	t.Skip() // TODO: writing to bytes.Buffer isn't thread-safe
	defer reset()

	buf := new(bytes.Buffer)
	stderr = buf
	defer func() { stderr = os.Stderr }()
	NewTask("error", 1, func(context.Context) error { return errors.New("oh noes") })
	NewTask("panic", 1, func(context.Context) error { panic("FIRE!") })

	MustRunTask("error")
	Wait("")
	MustRunTask("panic")
	Wait("")

	want := "bgrun: error running task \"error\": oh noes\nbgrun: error running task \"panic\": FIRE!\n"
	if buf.String() != want {
		t.Errorf("\nwant: %q\nhave: %q", want, buf.String())
	}
}

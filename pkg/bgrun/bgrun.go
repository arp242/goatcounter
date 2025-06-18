// Package bgrun runs jobs in the background.
package bgrun

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"sort"
	"sync"
	"time"
)

type (
	task struct {
		name   string
		maxPar int
		fun    func(context.Context) error
	}
	job struct {
		task      task
		wg        sync.WaitGroup
		num       int
		instances []*jobInstance
	}
	jobInstance struct {
		from    string
		started time.Time
	}
	Job struct {
		Task    string        // Task name
		Started time.Time     // When the job was started.
		Took    time.Duration // How long the job took to run.
		From    string        // Location where the job was started from.
	}
	Runner struct {
		ctx     context.Context
		cancel  context.CancelFunc
		maxHist int
		depth   int
		mu      sync.Mutex
		tasks   map[string]task
		jobs    map[string]*job
		hist    []Job
		logger  func(task string, err error)
	}
)

type ErrTooManyJobs struct {
	Task string
	Num  int
}

func (e ErrTooManyJobs) Error() string {
	return fmt.Sprintf("bgrun.Run: task %q has %d jobs already", e.Task, e.Num)
}

func NewRunner(logErr func(task string, err error)) *Runner {
	ctx, cancel := context.WithCancel(context.Background())
	return &Runner{
		ctx:     ctx,
		cancel:  cancel,
		maxHist: 100,
		depth:   2,
		tasks:   make(map[string]task),
		jobs:    make(map[string]*job),
		hist:    make([]Job, 0, 100),
		logger:  logErr,
	}
}

// NewTask registers a new task.
func (r *Runner) NewTask(name string, maxPar int, f func(context.Context) error) {
	if maxPar < 1 {
		maxPar = 1
	}
	if name == "" {
		panic("bgrun.New: name cannot be an empty string")
	}
	if f == nil {
		panic("bgrun.New: function cannot be nil")
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.tasks[name]; ok {
		panic(fmt.Sprintf("bgrun.New: task %q already exists", name))
	}
	r.tasks[name] = task{
		name:   name,
		maxPar: maxPar,
		fun:    f,
	}
}

func (r *Runner) Reset() {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.maxHist = 100
	r.tasks = make(map[string]task)
	r.jobs = make(map[string]*job)
	r.hist = make([]Job, 0, r.maxHist)
}

// Run a new job.
func (r *Runner) Run(name string, fun func(context.Context) error) error {
	return r.run(name, fun)
}

// MustRun behaves like [Run], but will panic on errors.
func (r *Runner) MustRun(name string, fun func(context.Context) error) {
	if err := r.run(name, fun); err != nil {
		panic(err)
	}
}

// RunFunction is like [Run], but the function doesn't support cancellation
// through context or error logging.
func (r *Runner) RunFunction(name string, fun func()) error {
	return r.Run(name, func(context.Context) error { fun(); return nil })
}

// MustRunFunction is like [RunFunction], but will panic on errors.
func (r *Runner) MustRunFunction(name string, fun func()) {
	if err := r.RunFunction(name, fun); err != nil {
		panic(err)
	}
}

// MustRunTask behaves like [RunTask], but will panic on errors.
func (r *Runner) MustRunTask(name string) {
	if err := r.run(name, nil); err != nil {
		panic(err)
	}
}

// RunTask runs a registered task.
func (r *Runner) RunTask(name string) error {
	return r.run(name, nil)
}

// Always call this function from both RunTask() and MustRunTask() so the stack
// trace is always identical.
func (r *Runner) run(name string, fun func(context.Context) error) error {
	isTask := fun == nil
	r.mu.Lock()
	defer r.mu.Unlock()

	t, ok := r.tasks[name]
	if !ok && isTask {
		return fmt.Errorf("bgrun.Run: no task %q", name)
	}

	j, ok := r.jobs[name]
	if isTask {
		if ok && j.num >= t.maxPar {
			return &ErrTooManyJobs{Task: name, Num: j.num}
		}
		fun = t.fun
	} else {
		t = task{name: name}
	}
	if !ok {
		j = &job{task: t}
		r.jobs[name] = j
	}

	i := len(j.instances)
	inst := jobInstance{
		started: time.Now(),
		from:    loc(r.depth),
	}
	j.instances = append(j.instances, &inst)

	j.wg.Add(1)
	go func() {
		defer func() {
			rec := recover()

			r.mu.Lock()
			r.hist = append(r.hist, Job{
				Task:    j.task.name,
				From:    inst.from,
				Started: inst.started,
				Took:    time.Since(inst.started),
			})
			if len(r.hist) > r.maxHist {
				r.hist = r.hist[len(r.hist)-r.maxHist:]
			}

			j.instances[i] = nil
			j.num--
			r.mu.Unlock()
			j.wg.Done()

			if rec != nil {
				switch rr := rec.(type) {
				case error:
					r.logger(name, rr)
				case string:
					r.logger(name, errors.New(rr))
				default:
					r.logger(name, fmt.Errorf("%s", rr))
				}
			}
		}()

		err := fun(r.ctx)
		if err != nil {
			r.logger(name, err)
		}
	}()
	return nil
}

// Wait for all running jobs for the task to finish.
//
// If name is an empty string it will wait for jobs for all tasks.
func (r *Runner) Wait(name string) {
	if name == "" {
		r.mu.Lock()
		var wg sync.WaitGroup
		wg.Add(len(r.jobs))
		for _, j := range r.jobs {
			j := j
			go func() {
				defer wg.Done()
				j.wg.Wait()
			}()
		}
		r.mu.Unlock()
		wg.Wait()
		return
	}

	r.mu.Lock()
	j, ok := r.jobs[name]
	r.mu.Unlock()
	if ok {
		j.wg.Wait()
	}
}

func (r *Runner) WaitFor(d time.Duration, name string) error {
	var (
		t    = time.NewTimer(d)
		done = make(chan struct{})
	)
	go func() {
		r.Wait(name)
		t.Stop()
		close(done)
	}()

	select {
	case <-t.C:
		r.cancel()
		return fmt.Errorf("bgrun.WaitFor: %w", context.DeadlineExceeded)
	case <-done:
		return nil
	}
}

// History gets the history. Only jobs that are finished running are added to
// the history.
//
// If newSize is >0 then it also sets the new history size (the default is 100).
// if newSize <0 history will be disabled.
func (r *Runner) History(newSize int) []Job {
	r.mu.Lock()
	defer r.mu.Unlock()

	cpy := make([]Job, len(r.hist))
	copy(cpy, r.hist)

	if newSize > 0 {
		r.maxHist = newSize
		if newSize < len(r.hist) {
			r.hist = make([]Job, newSize)
			copy(r.hist, cpy)
		}
	}
	if newSize < 0 {
		r.maxHist = 0
		r.hist = nil
	}

	return cpy
}

// Running returns all running jobs.
func (r *Runner) Running() []Job {
	r.mu.Lock()
	defer r.mu.Unlock()

	l := make([]Job, 0, len(r.jobs))
	for _, j := range r.jobs {
		for _, inst := range j.instances {
			if inst != nil {
				l = append(l, Job{
					Task:    j.task.name,
					Started: inst.started,
					From:    inst.from,
				})
			}
		}
	}
	sort.Slice(l, func(i, j int) bool { return l[i].Started.Before(l[j].Started) })
	return l
}

// loc gets a location in the stack trace. Use 0 for the current location; 1 for
// one up, etc.
func loc(n int) string {
	_, file, line, ok := runtime.Caller(n + 1)
	if !ok {
		file = "???"
		line = 0
	}

	short := file
	for i := len(file) - 1; i > 0; i-- {
		if file[i] == '/' {
			short = file[i+1:]
			break
		}
	}
	file = short

	return fmt.Sprintf("%v:%v", file, line)
}

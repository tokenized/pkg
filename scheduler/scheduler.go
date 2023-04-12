package scheduler

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/tokenized/logger"
)

const (
	SubSystem = "Scheduler" // For logger
)

var (
	NotFound = errors.New("Task not found")
)

// Scheduler provides the ability to schedule tasks to run at when they are ready.
type Scheduler struct {
	jobs          []Task
	lock          sync.Mutex
	isRunning     bool
	stopRequested bool
}

// Task provides an interface that tells Scheduler when and how to do something.
type Task interface {
	// IsReady returns true when a job should be executed.
	IsReady(ctx context.Context) bool

	// Run executes the job.
	Run(ctx context.Context)

	// IsComplete returns true when a job should be removed from the scheduler.
	IsComplete(ctx context.Context) bool

	// Equal returns true if another task matches it. Used to cancel jobs.
	Equal(other Task) bool
}

// ScheduleJob adds a task to the scheduler.
func (sch *Scheduler) ScheduleJob(ctx context.Context, job Task) error {
	sch.lock.Lock()
	defer sch.lock.Unlock()
	sch.jobs = append(sch.jobs, job)
	return nil
}

// CancelJob removes a job from the scheduler. The task passed in just needs to be equivalent based
//   on the task's Equal function.
func (sch *Scheduler) CancelJob(ctx context.Context, task Task) error {
	sch.lock.Lock()
	defer sch.lock.Unlock()
	for i, existing := range sch.jobs {
		if existing.Equal(task) {
			sch.jobs = append(sch.jobs[:i], sch.jobs[i+1:]...)
			return nil
		}
	}
	return NotFound
}

// Run monitors tasks and runs them when they are ready.
func (sch *Scheduler) Run(ctx context.Context) error {
	sch.lock.Lock()
	sch.isRunning = true
	for !sch.stopRequested {
		// Check and run tasks
		for i, job := range sch.jobs {
			if job.IsReady(ctx) {
				job.Run(ctx)
				if job.IsComplete(ctx) {
					sch.jobs = append(sch.jobs[:i], sch.jobs[i+1:]...)
					break // Modified list being iterated
				}
			}
		}

		// Unlock for sleep
		sch.lock.Unlock()
		time.Sleep(500 * time.Millisecond)
		sch.lock.Lock()
	}
	sch.isRunning = false
	sch.lock.Unlock()
	return nil
}

// stillRunning returns true if the scheduler is still running.
func (sch *Scheduler) stillRunning() bool {
	sch.lock.Lock()
	defer sch.lock.Unlock()
	return sch.isRunning
}

// Stop requests Run finish and waits for it to finish.
func (sch *Scheduler) Stop(ctx context.Context) error {
	ctx = logger.ContextWithLogSubSystem(ctx, SubSystem)
	sch.lock.Lock()
	sch.stopRequested = true
	sch.lock.Unlock()

	count := 0
	for sch.stillRunning() {
		time.Sleep(200 * time.Millisecond)
		if count > 30 { // 3 seconds
			logger.Info(ctx, "Waiting for scheduler to stop")
			count = 0
		}
		count++
	}
	return nil
}

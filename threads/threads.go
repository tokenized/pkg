package threads

import (
	"context"
	"sync"
	"time"

	"github.com/tokenized/pkg/logger"

	"github.com/pkg/errors"
)

// Thread adds common functionality around go routines to manage them. It provides the ability to
// monitor when they complete through WaitGroup or a select on a channel.
// The function to call only takes a context and a "read only" interrupt channel. The interrupt
// channel can be used within the function in a select that will trigger the function to return.
// If the function reads from a data channel then a select can be used to read from the data channel
// and the interrupt channel so it can be done within the same go routine.
type Thread struct {
	name string

	interruptFunction ThreadInterruptFunction
	interrupt         chan interface{}

	stopFunction ThreadStopFunction
	stop         *AtomicFlag

	frequency    time.Duration
	taskFunction TaskFunction

	noStopFunction TaskFunction

	onComplete ThreadCompleteFunction
	complete   *chan interface{}
	wait       *sync.WaitGroup
	err        error
	isComplete bool
	wasStopped bool

	sync.Mutex
}

type Threads []*Thread

// ThreadInterruptFunction should select on interrupt and end the function when it is if not before.
type ThreadInterruptFunction func(ctx context.Context, interrupt <-chan interface{}) error

// ThreadStopFunction should periodically check if stop.IsSet and end the function when it is if not
// before.
type ThreadStopFunction func(ctx context.Context, stop *AtomicFlag) error

// ThreadCompleteFunction can be configured to be called when a thread completes. It passes the
// result error of the thread to the function.
type ThreadCompleteFunction func(ctx context.Context, err error)

// TaskFunction is a function that performs a task. It is used to perform tasks periodically.
type TaskFunction func(ctx context.Context) error

func (ts Threads) Start(ctx context.Context) {
	for _, thread := range ts {
		thread.Start(ctx)
	}
}

func (ts Threads) Stop(ctx context.Context) {
	for _, thread := range ts {
		thread.Stop(ctx)
	}
}

func (ts Threads) Errors() []error {
	var result []error
	for _, thread := range ts {
		if err := thread.Error(); err != nil {
			result = append(result, err)
		}
	}

	return result
}

// NewThread creates a thread around a function that can be aborted/stopped by closing an interrupt
// channel.
func NewThread(name string, function ThreadInterruptFunction) *Thread {
	// For interrupt use buffered with size of one so it doesn't wait on write if there is no reader
	// waiting
	return &Thread{
		name:              name,
		interruptFunction: function,
		interrupt:         make(chan interface{}, 1),
	}
}

// NewStopThread creates a thread around a function that can be aborted/stopped by setting an atomic
// flag.
func NewStopThread(name string, function ThreadStopFunction) *Thread {
	return &Thread{
		name:         name,
		stopFunction: function,
		stop:         NewAtomicFlag(),
	}
}

func NewPeriodicTask(name string, frequency time.Duration, function TaskFunction) *Thread {
	return &Thread{
		name:         name,
		frequency:    frequency,
		taskFunction: function,
		interrupt:    make(chan interface{}, 1),
	}
}

func NewThreadWithoutStop(name string, function TaskFunction) *Thread {
	return &Thread{
		name:           name,
		noStopFunction: function,
	}
}

// SetWait specifies a wait to add to when starting the function and to specify "done" to when it
// completes.
func (t *Thread) SetWait(wait *sync.WaitGroup) {
	t.Lock()
	defer t.Unlock()

	t.wait = wait
}

// GetWait creates a wait to add to when starting the function and to specify "done" to when it
// completes.
func (t *Thread) GetWait() *sync.WaitGroup {
	t.Lock()
	defer t.Unlock()

	t.wait = &sync.WaitGroup{}
	return t.wait
}

// GetCompleteChannel returns a channel that will be closed when the function completes. Return a
// "read only" channel because it should only be read from in a select to trigger shutdown.
func (t *Thread) GetCompleteChannel() <-chan interface{} {
	t.Lock()
	defer t.Unlock()

	// use buffered with size of one so it doesn't wait on write if there is no reader waiting
	complete := make(chan interface{}, 1)
	t.complete = &complete
	return complete
}

func (t *Thread) SetOnComplete(onComplete ThreadCompleteFunction) {
	t.Lock()
	defer t.Unlock()

	t.onComplete = onComplete
}

func (t *Thread) Start(ctx context.Context) {
	if t.wait != nil {
		t.wait.Add(1)
	}

	caller := logger.GetCaller(1) // use caller of thread start rather than thread file

	t.Lock()
	name := t.name
	t.Unlock()

	go func() {
		logger.LogDepthWithFields(ctx, logger.LevelDebug, caller, nil, "Starting: %s", name)
		var err error
		if t.interruptFunction != nil {
			err = t.interruptFunction(ctx, t.interrupt)
		} else if t.stopFunction != nil {
			err = t.stopFunction(ctx, t.stop)
		} else if t.noStopFunction != nil {
			err = t.noStopFunction(ctx)
		} else if t.taskFunction != nil {
			err = t.runPeriodic(ctx)
		}

		if err == nil {
			logger.LogDepthWithFields(ctx, logger.LevelDebug, caller, nil, "Finished: %s", name)
		} else if errors.Cause(err) == Interrupted {
			logger.LogDepthWithFields(ctx, logger.LevelDebug, caller, nil, "Finished: %s : %s",
				name, err)
		} else {
			logger.LogDepthWithFields(ctx, logger.LevelWarn, caller, nil, "Finished: %s: %s", name,
				err)
		}

		t.Lock()
		t.err = err
		if t.onComplete != nil {
			t.onComplete(ctx, err)
		}
		if t.complete != nil {
			close(*t.complete)
		}
		t.isComplete = true
		t.Unlock()

		if t.wait != nil {
			t.wait.Done()
		}
	}()
}

func (t *Thread) runPeriodic(ctx context.Context) error {
	for {
		select {
		case <-t.interrupt:
			return nil

		case <-time.After(t.frequency):
			if err := t.taskFunction(ctx); err != nil {
				return err
			}
		}
	}
}

func (t *Thread) Stop(ctx context.Context) {
	t.Lock()
	defer t.Unlock()

	if t.wasStopped {
		return
	}

	if t.interrupt != nil {
		close(t.interrupt)
	} else if t.stop != nil {
		t.stop.Set()
	} else {
		logger.Error(ctx, "Thread does not support stop function")
	}

	t.wasStopped = true
}

func (t *Thread) IsComplete() bool {
	t.Lock()
	defer t.Unlock()

	return t.isComplete
}

func (t *Thread) Error() error {
	if t == nil {
		return nil
	}

	t.Lock()
	defer t.Unlock()

	return errors.Wrap(t.err, t.name)
}

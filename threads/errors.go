package threads

import (
	"strings"

	"github.com/pkg/errors"
)

var (
	// Interrupted means the function was interrupted by the interrupt channel and the function
	// did not finish. It can be used to ensure that if a calling function uses the caller's
	// interrupt channel then the calling function will still return if it receives the interrupt
	// and the child function is interrupted.
	Interrupted = errors.New("Interrupted")

	// ErrCombined is used to combine multiple errors when more than one thread had issues.
	ErrCombined = errors.New("Combined errors")
)

func CombineErrors(errs ...error) error {
	var list []error
	allWereInterrupted := true
	for _, err := range errs {
		if err == nil {
			continue // no error
		}

		if errors.Cause(err) != Interrupted {
			allWereInterrupted = false
		}

		if len(list) == 0 {
			list = []error{err} // set first
			continue
		}

		// combine with previous
		list = append(list, err)
	}

	if len(list) == 0 {
		return nil
	}

	if len(list) == 1 {
		return list[0]
	}

	stringList := make([]string, len(list))
	for i, err := range list {
		stringList[i] = err.Error()
	}

	if allWereInterrupted {
		// All errors were interrupted so keep the parent type the same so it can be recognized.
		return errors.Wrap(Interrupted, strings.Join(stringList, "|"))
	}

	return errors.Wrap(ErrCombined, strings.Join(stringList, "|"))
}

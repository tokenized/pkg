package threads

import (
	"context"
)

type Stopper interface {
	Stop(context.Context)
}

type StopCombiner []Stopper

func (s *StopCombiner) Add(stopper Stopper) {
	*s = append(*s, stopper)
}

func (s StopCombiner) Stop(ctx context.Context) {
	for _, stopper := range s {
		stopper.Stop(ctx)
	}
}

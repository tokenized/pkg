package threads

import (
	"context"
	"testing"
	"time"
)

func Test_Periodic(t *testing.T) {
	ctx := context.Background()

	i := 0
	thread := NewPeriodicTask("Test Periodic", time.Second, func(ctx context.Context) error {
		t.Logf("Message %d : %s", i, time.Now())
		i++
		return nil
	})
	wait := thread.GetWait()

	thread.Start(ctx)

	time.Sleep(3100 * time.Millisecond)

	thread.Stop(ctx)
	wait.Wait()

	if i != 3 {
		t.Errorf("Wrong increment value : got %d, want %d", i, 3)
	}
}

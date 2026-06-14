package signal_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/davidtobonm/heracles/internal/signal"
)

// fakeNotifier delivers signals on demand for deterministic tests, in place
// of real OS signals.
type fakeNotifier struct {
	ch chan<- os.Signal
}

func (notifier *fakeNotifier) Notify(ch chan<- os.Signal) { notifier.ch = ch }
func (notifier *fakeNotifier) Stop(chan<- os.Signal)      {}

func (notifier *fakeNotifier) send() {
	notifier.ch <- os.Interrupt
}

func TestFirstInterruptRequestsStopWithoutCancellingHardContext(t *testing.T) {
	t.Parallel()

	notifier := &fakeNotifier{}
	interrupt, stop := signal.Watch(context.Background(), notifier)
	defer stop()

	notifier.send()
	waitUntil(t, interrupt.StopRequested)

	select {
	case <-interrupt.Hard.Done():
		t.Fatal("Hard context cancelled after only one interrupt")
	default:
	}
}

func TestSecondInterruptCancelsHardContext(t *testing.T) {
	t.Parallel()

	notifier := &fakeNotifier{}
	interrupt, stop := signal.Watch(context.Background(), notifier)
	defer stop()

	notifier.send()
	waitUntil(t, interrupt.StopRequested)
	notifier.send()

	select {
	case <-interrupt.Hard.Done():
	case <-time.After(time.Second):
		t.Fatal("Hard context not cancelled after second interrupt")
	}
}

func TestStopCancelsHardContext(t *testing.T) {
	t.Parallel()

	notifier := &fakeNotifier{}
	interrupt, stop := signal.Watch(context.Background(), notifier)
	stop()

	select {
	case <-interrupt.Hard.Done():
	case <-time.After(time.Second):
		t.Fatal("Hard context not cancelled after Stop")
	}
}

func waitUntil(t *testing.T, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for !condition() {
		if time.Now().After(deadline) {
			t.Fatal("condition not met before deadline")
		}
		time.Sleep(time.Millisecond)
	}
}

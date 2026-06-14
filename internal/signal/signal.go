// Package signal turns OS interrupt signals into the two-stage durable
// shutdown described by ADR 0030: the first SIGINT/SIGTERM asks running work
// to stop at its next durable boundary and leave resumable state; a second
// SIGINT/SIGTERM cancels a hard context so in-flight subprocesses are
// terminated immediately.
package signal

import (
	"context"
	"os"
	"sync/atomic"
)

// Notifier reports interrupt signals, decoupled from os/signal for tests.
type Notifier interface {
	// Notify delivers signals to ch until Stop is called.
	Notify(ch chan<- os.Signal)
	// Stop stops delivering signals to ch.
	Stop(ch chan<- os.Signal)
}

// Interrupt carries the graceful-stop flag and the hard-cancellation
// context derived from OS interrupt signals.
type Interrupt struct {
	// Hard is cancelled on the second interrupt. Subprocesses started with
	// exec.CommandContext(Hard, ...) are killed immediately.
	Hard context.Context

	stopped atomic.Bool
}

// StopRequested reports whether the first interrupt has been received. Long
// running work should check this between durable boundaries and, if true,
// return early leaving resumable state.
func (interrupt *Interrupt) StopRequested() bool {
	return interrupt.stopped.Load()
}

type stopRequestedKey struct{}

// WithStopRequested attaches a graceful-stop check to ctx. Long running work
// reads it back with StopRequested.
func WithStopRequested(ctx context.Context, stopRequested func() bool) context.Context {
	return context.WithValue(ctx, stopRequestedKey{}, stopRequested)
}

// StopRequested reports whether ctx carries a graceful-stop request that has
// been triggered. It returns false if ctx carries no such request.
func StopRequested(ctx context.Context) bool {
	stopRequested, ok := ctx.Value(stopRequestedKey{}).(func() bool)
	return ok && stopRequested()
}

// Watch starts watching for interrupt signals delivered by notifier and
// returns an Interrupt plus a function that stops watching and releases
// resources. The first delivered signal sets StopRequested; the second
// cancels Hard.
func Watch(parent context.Context, notifier Notifier) (*Interrupt, func()) {
	hard, cancel := context.WithCancel(parent)
	interrupt := &Interrupt{Hard: hard}

	ch := make(chan os.Signal, 2)
	notifier.Notify(ch)
	done := make(chan struct{})
	go func() {
		for {
			select {
			case <-ch:
				if !interrupt.stopped.CompareAndSwap(false, true) {
					cancel()
					return
				}
			case <-done:
				return
			}
		}
	}()

	stop := func() {
		notifier.Stop(ch)
		close(done)
		cancel()
	}
	return interrupt, stop
}

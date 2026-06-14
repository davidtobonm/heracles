package signal

import (
	"os"
	ossignal "os/signal"
	"syscall"
)

// Signals are the OS signals that trigger graceful and then hard
// interruption: SIGINT (Ctrl-C) and SIGTERM.
var Signals = []os.Signal{os.Interrupt, syscall.SIGTERM}

// OSNotifier delivers real OS signals via os/signal.
type OSNotifier struct{}

// Notify registers ch to receive Signals.
func (OSNotifier) Notify(ch chan<- os.Signal) {
	ossignal.Notify(ch, Signals...)
}

// Stop stops delivering signals to ch.
func (OSNotifier) Stop(ch chan<- os.Signal) {
	ossignal.Stop(ch)
}

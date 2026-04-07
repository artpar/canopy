package transport

import (
	"canopy/protocol"
	"runtime"
	"testing"
	"time"
)

// goroutineLeakCheck records the goroutine count at call time and asserts
// no growth after the returned cleanup runs. Call at test start, defer the result.
func goroutineLeakCheck(t *testing.T) func() {
	t.Helper()
	before := runtime.NumGoroutine()
	return func() {
		t.Helper()
		deadline := time.Now().Add(200 * time.Millisecond)
		for time.Now().Before(deadline) {
			after := runtime.NumGoroutine()
			if after <= before {
				return
			}
			runtime.Gosched()
			time.Sleep(10 * time.Millisecond)
		}
		after := runtime.NumGoroutine()
		if after > before {
			t.Errorf("goroutine leak: before=%d after=%d", before, after)
		}
	}
}

// drainWithTimeout drains a message channel with a timeout.
func drainWithTimeout(t *testing.T, ch <-chan *protocol.Message, timeout time.Duration) []*protocol.Message {
	t.Helper()
	var msgs []*protocol.Message
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	for {
		select {
		case msg, ok := <-ch:
			if !ok {
				return msgs
			}
			msgs = append(msgs, msg)
		case <-timer.C:
			t.Fatal("timeout draining messages")
			return msgs
		}
	}
}

// drainErrorsWithTimeout drains an error channel with a timeout.
func drainErrorsWithTimeout(t *testing.T, ch <-chan error, timeout time.Duration) []error {
	t.Helper()
	var errs []error
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	for {
		select {
		case err, ok := <-ch:
			if !ok {
				return errs
			}
			errs = append(errs, err)
		case <-timer.C:
			t.Fatal("timeout draining errors")
			return errs
		}
	}
}

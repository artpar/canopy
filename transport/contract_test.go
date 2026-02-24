package transport

import (
	"jview/protocol"
	"path/filepath"
	"testing"
	"time"
)

// RunTransportContractTests is a reusable suite that any Transport implementation must pass.
func RunTransportContractTests(t *testing.T, factory func() Transport) {
	t.Run("MessagesReturnsNonNil", func(t *testing.T) {
		tr := factory()
		if tr.Messages() == nil {
			t.Fatal("Messages() returned nil")
		}
	})

	t.Run("ErrorsReturnsNonNil", func(t *testing.T) {
		tr := factory()
		if tr.Errors() == nil {
			t.Fatal("Errors() returned nil")
		}
	})

	t.Run("MessagesClosesAfterDrain", func(t *testing.T) {
		defer goroutineLeakCheck(t)()
		tr := factory()
		tr.Start()

		timer := time.NewTimer(5 * time.Second)
		defer timer.Stop()
		for {
			select {
			case _, ok := <-tr.Messages():
				if !ok {
					return // closed
				}
			case <-timer.C:
				t.Fatal("Messages channel did not close")
			}
		}
	})

	t.Run("ErrorsClosesAfterDrain", func(t *testing.T) {
		defer goroutineLeakCheck(t)()
		tr := factory()
		tr.Start()

		// Drain messages first
		drainWithTimeout(t, tr.Messages(), 5*time.Second)

		// Then errors must close
		timer := time.NewTimer(time.Second)
		defer timer.Stop()
		for {
			select {
			case _, ok := <-tr.Errors():
				if !ok {
					return
				}
			case <-timer.C:
				t.Fatal("Errors channel did not close")
			}
		}
	})

	t.Run("SendActionDoesNotPanic", func(t *testing.T) {
		tr := factory()
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("SendAction panicked: %v", r)
			}
		}()
		tr.SendAction("surface1", &protocol.Action{
			Type: "serverAction",
			Name: "test",
		}, map[string]interface{}{"key": "value"})
	})

	t.Run("StopIsIdempotent", func(t *testing.T) {
		defer goroutineLeakCheck(t)()
		tr := factory()
		tr.Start()

		// First Stop
		tr.Stop()
		// Second Stop must not panic
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("double Stop() panicked: %v", r)
			}
		}()
		tr.Stop()

		// Drain to let goroutine exit
		for range tr.Messages() {
		}
		for range tr.Errors() {
		}
	})
}

func TestFileTransportContract(t *testing.T) {
	RunTransportContractTests(t, func() Transport {
		return NewFileTransport(filepath.Join(fixtureDir(), "hello.jsonl"))
	})
}

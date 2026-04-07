package engine

import (
	"canopy/renderer"
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

// assertCreated finds a created view by component ID or fails.
func assertCreated(t *testing.T, mock *renderer.MockRenderer, componentID string) renderer.CreatedView {
	t.Helper()
	for _, c := range mock.Created {
		if c.Node.ComponentID == componentID {
			return c
		}
	}
	t.Fatalf("component %q not created", componentID)
	return renderer.CreatedView{}
}

// assertUpdated finds an updated view by component ID after a given index, or fails.
func assertUpdated(t *testing.T, mock *renderer.MockRenderer, componentID string, afterIndex int) renderer.UpdatedView {
	t.Helper()
	for i := afterIndex; i < len(mock.Updated); i++ {
		if mock.Updated[i].Node != nil && mock.Updated[i].Node.ComponentID == componentID {
			return mock.Updated[i]
		}
	}
	t.Fatalf("component %q not updated after index %d", componentID, afterIndex)
	return renderer.UpdatedView{}
}

// newTestSession returns a fresh Session with a MockRenderer and MockDispatcher.
func newTestSession() (*Session, *renderer.MockRenderer) {
	mock := renderer.NewMockRenderer()
	disp := &renderer.MockDispatcher{}
	sess := NewSession(mock, disp)
	return sess, mock
}

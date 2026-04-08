package engine

import (
	"sync/atomic"
	"testing"
	"time"
)

func TestThrottleFiresImmediately(t *testing.T) {
	th := NewThrottler(100, "throttle")
	var count int32
	th.Call(func() { atomic.AddInt32(&count, 1) })
	if atomic.LoadInt32(&count) != 1 {
		t.Fatal("throttle should fire immediately on first call")
	}
}

func TestThrottleDropsWithinInterval(t *testing.T) {
	th := NewThrottler(100, "throttle")
	var count int32
	th.Call(func() { atomic.AddInt32(&count, 1) })
	th.Call(func() { atomic.AddInt32(&count, 1) })
	th.Call(func() { atomic.AddInt32(&count, 1) })
	if atomic.LoadInt32(&count) != 1 {
		t.Fatalf("expected 1 call, got %d", atomic.LoadInt32(&count))
	}
}

func TestThrottleFiresAfterInterval(t *testing.T) {
	th := NewThrottler(50, "throttle")
	var count int32
	th.Call(func() { atomic.AddInt32(&count, 1) })
	time.Sleep(60 * time.Millisecond)
	th.Call(func() { atomic.AddInt32(&count, 1) })
	if atomic.LoadInt32(&count) != 2 {
		t.Fatalf("expected 2 calls, got %d", atomic.LoadInt32(&count))
	}
}

func TestDebounceDelaysFiring(t *testing.T) {
	th := NewThrottler(50, "debounce")
	var count int32
	th.Call(func() { atomic.AddInt32(&count, 1) })
	// Should not fire immediately
	if atomic.LoadInt32(&count) != 0 {
		t.Fatal("debounce should not fire immediately")
	}
	// Wait for debounce to fire
	time.Sleep(80 * time.Millisecond)
	if atomic.LoadInt32(&count) != 1 {
		t.Fatalf("expected 1 call after debounce, got %d", atomic.LoadInt32(&count))
	}
}

func TestDebounceResetsOnNewCall(t *testing.T) {
	th := NewThrottler(50, "debounce")
	var count int32
	th.Call(func() { atomic.AddInt32(&count, 1) })
	time.Sleep(30 * time.Millisecond)
	// Reset by calling again before debounce fires
	th.Call(func() { atomic.AddInt32(&count, 1) })
	time.Sleep(30 * time.Millisecond)
	// Should still not have fired
	if atomic.LoadInt32(&count) != 0 {
		t.Fatal("debounce should not fire during reset period")
	}
	// Wait for full debounce after last call
	time.Sleep(40 * time.Millisecond)
	if atomic.LoadInt32(&count) != 1 {
		t.Fatalf("expected 1 call after debounce, got %d", atomic.LoadInt32(&count))
	}
}

func TestDebounceFiresLatestFn(t *testing.T) {
	th := NewThrottler(50, "debounce")
	var result int32
	th.Call(func() { atomic.StoreInt32(&result, 1) })
	th.Call(func() { atomic.StoreInt32(&result, 2) })
	th.Call(func() { atomic.StoreInt32(&result, 3) })
	time.Sleep(80 * time.Millisecond)
	if atomic.LoadInt32(&result) != 3 {
		t.Fatalf("expected latest fn (3), got %d", atomic.LoadInt32(&result))
	}
}

func TestStopCancelsPendingDebounce(t *testing.T) {
	th := NewThrottler(50, "debounce")
	var count int32
	th.Call(func() { atomic.AddInt32(&count, 1) })
	th.Stop()
	time.Sleep(80 * time.Millisecond)
	if atomic.LoadInt32(&count) != 0 {
		t.Fatal("Stop should cancel pending debounce")
	}
}

func TestStopOnThrottleIsNoop(t *testing.T) {
	th := NewThrottler(50, "throttle")
	var count int32
	th.Call(func() { atomic.AddInt32(&count, 1) })
	th.Stop() // should not panic or affect anything
	if atomic.LoadInt32(&count) != 1 {
		t.Fatal("Stop on throttle should be a no-op")
	}
}

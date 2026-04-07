package mcp

import (
	"canopy/jlog"
	"canopy/renderer"
	"runtime"
)

// dispatchSync runs fn on the main thread via the dispatcher and blocks until it returns.
func dispatchSync[T any](disp renderer.Dispatcher, fn func() T) T {
	ch := make(chan T, 1)
	disp.RunOnMain(func() {
		defer func() {
			if r := recover(); r != nil {
				buf := make([]byte, 4096)
				n := runtime.Stack(buf, false)
				jlog.Errorf("mcp", "", "dispatch panic recovered: %v\n%s", r, buf[:n])
				var zero T
				ch <- zero
			}
		}()
		ch <- fn()
	})
	return <-ch
}

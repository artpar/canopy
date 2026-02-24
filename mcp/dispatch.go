package mcp

import "jview/renderer"

// dispatchSync runs fn on the main thread via the dispatcher and blocks until it returns.
func dispatchSync[T any](disp renderer.Dispatcher, fn func() T) T {
	ch := make(chan T, 1)
	disp.RunOnMain(func() { ch <- fn() })
	return <-ch
}

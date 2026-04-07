package darwin

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework Cocoa

#include "dispatch.h"
*/
import "C"
import (
	"canopy/jlog"
	"runtime"
	"runtime/cgo"
)

// Dispatcher implements renderer.Dispatcher using dispatch_async(main_queue).
type Dispatcher struct{}

func NewDispatcher() *Dispatcher {
	return &Dispatcher{}
}

// RunOnMain schedules fn on the macOS main thread.
func (d *Dispatcher) RunOnMain(fn func()) {
	h := cgo.NewHandle(fn)
	C.JVDispatchMainAsync(C.uintptr_t(h))
}

//export goDispatchCallback
func goDispatchCallback(handle C.uintptr_t) {
	h := cgo.Handle(uintptr(handle))
	fn := h.Value().(func())
	h.Delete()
	defer func() {
		if r := recover(); r != nil {
			buf := make([]byte, 4096)
			n := runtime.Stack(buf, false)
			jlog.Errorf("darwin", "", "dispatch panic recovered: %v\n%s", r, buf[:n])
		}
	}()
	fn()
}

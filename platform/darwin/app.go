package darwin

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework Cocoa

#include <stdlib.h>
#include "app.h"
*/
import "C"
import "unsafe"

// AppInit initializes NSApplication. Must be called from main thread.
func AppInit() {
	C.JVAppInit()
}

// AppRun starts the NSApplication run loop. Blocks forever. Must be on main thread.
func AppRun() {
	C.JVAppRun()
}

// AppStop stops the NSApplication run loop.
func AppStop() {
	C.JVAppStop()
}

// AppRunUntilIdle processes all pending events and returns. Used by test mode
// to let Auto Layout compute frames before running assertions.
func AppRunUntilIdle() {
	C.JVAppRunUntilIdle()
}

// ForceLayout forces a layout pass on a surface's window content view.
func ForceLayout(surfaceID string) {
	cSID := C.CString(surfaceID)
	defer C.free(unsafe.Pointer(cSID))
	C.JVForceLayout(cSID)
}

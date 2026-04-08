package darwin

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework Cocoa

#include <stdlib.h>
#include "eventmonitor.h"
*/
import "C"
import (
	"canopy/renderer"
	"unsafe"
)

// eventMonitorEvents lists event types handled by the native event monitor.
// These are installed/updated generically in CreateView/UpdateView rather than
// by component-specific code.
var eventMonitorEvents = map[string]bool{
	"mouseEnter":  true,
	"mouseLeave":  true,
	"doubleClick": true,
	"rightClick":  true,
	"focus":       true,
	"blur":        true,
	"keyDown":     true,
	"keyUp":       true,
	"magnify":     true,
	"rotate":      true,
	"scrollWheel": true,
}

func installEventMonitor(handle renderer.ViewHandle, eventName string, callbackID uint64) {
	cName := C.CString(eventName)
	defer C.free(unsafe.Pointer(cName))
	C.JVInstallEventMonitor(unsafe.Pointer(handle), cName, C.uint64_t(callbackID))
}

func updateEventMonitorCallbackID(handle renderer.ViewHandle, eventName string, callbackID uint64) {
	cName := C.CString(eventName)
	defer C.free(unsafe.Pointer(cName))
	C.JVUpdateEventMonitorCallbackID(unsafe.Pointer(handle), cName, C.uint64_t(callbackID))
}

func removeAllEventMonitors(handle renderer.ViewHandle) {
	C.JVRemoveAllEventMonitors(unsafe.Pointer(handle))
}

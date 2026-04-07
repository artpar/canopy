package darwin

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework Cocoa

#include "clickgesture.h"
*/
import "C"
import (
	"canopy/renderer"
	"unsafe"
)

func attachClickGesture(handle renderer.ViewHandle, callbackID uint64) {
	C.JVAttachClickGesture(unsafe.Pointer(handle), C.uint64_t(callbackID))
}

func updateClickGestureCallbackID(handle renderer.ViewHandle, callbackID uint64) {
	C.JVUpdateClickGestureCallbackID(unsafe.Pointer(handle), C.uint64_t(callbackID))
}

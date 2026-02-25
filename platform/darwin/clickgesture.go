package darwin

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework Cocoa

#include "clickgesture.h"
*/
import "C"
import (
	"jview/renderer"
	"unsafe"
)

func attachClickGesture(handle renderer.ViewHandle, callbackID uint64) {
	C.JVAttachClickGesture(unsafe.Pointer(handle), C.uint64_t(callbackID))
}

package darwin

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework Cocoa

#include <stdlib.h>
#include "datetimeinput.h"
*/
import "C"
import (
	"canopy/renderer"
	"unsafe"
)

func createDateTimeInputView(node *renderer.RenderNode, surfaceID string) renderer.ViewHandle {
	cValue := C.CString(node.Props.DateValue)
	defer C.free(unsafe.Pointer(cValue))

	var cbID uint64
	if id, ok := node.Callbacks["datechange"]; ok {
		cbID = uint64(id)
	}

	ptr := C.JVCreateDateTimeInput(
		C.bool(node.Props.EnableDate),
		C.bool(node.Props.EnableTime),
		cValue,
		C.uint64_t(cbID),
	)
	return renderer.ViewHandle(uintptr(ptr))
}

func updateDateTimeInputView(handle renderer.ViewHandle, node *renderer.RenderNode) {
	cValue := C.CString(node.Props.DateValue)
	defer C.free(unsafe.Pointer(cValue))

	C.JVUpdateDateTimeInput(
		unsafe.Pointer(handle),
		C.bool(node.Props.EnableDate),
		C.bool(node.Props.EnableTime),
		cValue,
	)
}

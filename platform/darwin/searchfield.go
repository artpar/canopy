package darwin

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework Cocoa

#include <stdlib.h>
#include <stdint.h>
#include "searchfield.h"
*/
import "C"
import (
	"canopy/renderer"
	"unsafe"
)

func createSearchFieldView(node *renderer.RenderNode, surfaceID string) renderer.ViewHandle {
	cPlaceholder := C.CString(node.Props.Placeholder)
	defer C.free(unsafe.Pointer(cPlaceholder))
	cValue := C.CString(node.Props.Value)
	defer C.free(unsafe.Pointer(cValue))

	var cbID uint64
	if id, ok := node.Callbacks["change"]; ok {
		cbID = uint64(id)
	}

	ptr := C.JVCreateSearchField(cPlaceholder, cValue, C.uint64_t(cbID))
	return renderer.ViewHandle(uintptr(ptr))
}

func updateSearchFieldView(handle renderer.ViewHandle, node *renderer.RenderNode) {
	cPlaceholder := C.CString(node.Props.Placeholder)
	defer C.free(unsafe.Pointer(cPlaceholder))
	cValue := C.CString(node.Props.Value)
	defer C.free(unsafe.Pointer(cValue))

	C.JVUpdateSearchField(unsafe.Pointer(handle), cPlaceholder, cValue)
}

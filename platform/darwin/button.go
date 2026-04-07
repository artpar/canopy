package darwin

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework Cocoa

#include <stdlib.h>
#include "button.h"
*/
import "C"
import (
	"canopy/renderer"
	"unsafe"
)

func createButtonView(node *renderer.RenderNode, surfaceID string) renderer.ViewHandle {
	cLabel := C.CString(node.Props.Label)
	defer C.free(unsafe.Pointer(cLabel))
	cStyle := C.CString(node.Props.Style)
	defer C.free(unsafe.Pointer(cStyle))

	var cbID uint64
	if id, ok := node.Callbacks["click"]; ok {
		cbID = uint64(id)
	}

	ptr := C.JVCreateButton(cLabel, cStyle, C.bool(node.Props.Disabled), C.uint64_t(cbID))
	return renderer.ViewHandle(uintptr(ptr))
}

func updateButtonView(handle renderer.ViewHandle, node *renderer.RenderNode) {
	cLabel := C.CString(node.Props.Label)
	defer C.free(unsafe.Pointer(cLabel))
	cStyle := C.CString(node.Props.Style)
	defer C.free(unsafe.Pointer(cStyle))

	C.JVUpdateButton(unsafe.Pointer(handle), cLabel, cStyle, C.bool(node.Props.Disabled))

	if cbID, ok := node.Callbacks["click"]; ok && cbID != 0 {
		C.JVUpdateButtonCallbackID(unsafe.Pointer(handle), C.uint64_t(cbID))
	}
}

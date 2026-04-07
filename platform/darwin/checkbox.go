package darwin

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework Cocoa

#include <stdlib.h>
#include "checkbox.h"
*/
import "C"
import (
	"canopy/renderer"
	"unsafe"
)

func createCheckBoxView(node *renderer.RenderNode, surfaceID string) renderer.ViewHandle {
	cLabel := C.CString(node.Props.Label)
	defer C.free(unsafe.Pointer(cLabel))

	var cbID uint64
	if id, ok := node.Callbacks["toggle"]; ok {
		cbID = uint64(id)
	}

	ptr := C.JVCreateCheckBox(cLabel, C.bool(node.Props.Checked), C.uint64_t(cbID))
	return renderer.ViewHandle(uintptr(ptr))
}

func updateCheckBoxView(handle renderer.ViewHandle, node *renderer.RenderNode) {
	cLabel := C.CString(node.Props.Label)
	defer C.free(unsafe.Pointer(cLabel))

	C.JVUpdateCheckBox(unsafe.Pointer(handle), cLabel, C.bool(node.Props.Checked))
}

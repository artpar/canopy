package darwin

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework Cocoa

#include <stdlib.h>
#include <stdbool.h>
#include "modal.h"
*/
import "C"
import (
	"canopy/renderer"
	"unsafe"
)

func createModalView(node *renderer.RenderNode, surfaceID string) renderer.ViewHandle {
	cTitle := C.CString(node.Props.Title)
	defer C.free(unsafe.Pointer(cTitle))
	cSID := C.CString(surfaceID)
	defer C.free(unsafe.Pointer(cSID))

	var cbID uint64
	if id, ok := node.Callbacks["dismiss"]; ok {
		cbID = uint64(id)
	}

	ptr := C.JVCreateModal(cTitle, C.bool(node.Props.Visible), cSID,
		C.int(node.Props.Width), C.int(node.Props.Height), C.uint64_t(cbID))
	return renderer.ViewHandle(uintptr(ptr))
}

func updateModalView(handle renderer.ViewHandle, node *renderer.RenderNode) {
	cTitle := C.CString(node.Props.Title)
	defer C.free(unsafe.Pointer(cTitle))

	C.JVUpdateModal(unsafe.Pointer(handle), cTitle, C.bool(node.Props.Visible))
}

func setModalChildren(parentHandle renderer.ViewHandle, childHandles []renderer.ViewHandle) {
	if len(childHandles) == 0 {
		C.JVModalSetChildren(unsafe.Pointer(parentHandle), nil, 0)
		return
	}

	ptrs := make([]unsafe.Pointer, len(childHandles))
	for i, h := range childHandles {
		ptrs[i] = unsafe.Pointer(h)
	}

	C.JVModalSetChildren(unsafe.Pointer(parentHandle), &ptrs[0], C.int(len(ptrs)))
}

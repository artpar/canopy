package darwin

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework Cocoa

#include <stdlib.h>
#include <stdint.h>
#include "outlineview.h"
*/
import "C"
import (
	"canopy/renderer"
	"unsafe"
)

func createOutlineView(node *renderer.RenderNode, surfaceID string) renderer.ViewHandle {
	cData := C.CString(node.Props.OutlineData)
	defer C.free(unsafe.Pointer(cData))
	cLabelKey := C.CString(node.Props.LabelKey)
	defer C.free(unsafe.Pointer(cLabelKey))
	cChildrenKey := C.CString(node.Props.ChildrenKey)
	defer C.free(unsafe.Pointer(cChildrenKey))
	cIconKey := C.CString(node.Props.IconKey)
	defer C.free(unsafe.Pointer(cIconKey))
	cIDKey := C.CString(node.Props.IDKey)
	defer C.free(unsafe.Pointer(cIDKey))
	cSelectedID := C.CString(node.Props.SelectedID)
	defer C.free(unsafe.Pointer(cSelectedID))
	cBadgeKey := C.CString(node.Props.BadgeKey)
	defer C.free(unsafe.Pointer(cBadgeKey))

	var cbID uint64
	if id, ok := node.Callbacks["select"]; ok {
		cbID = uint64(id)
	}

	ptr := C.JVCreateOutlineView(cData, cLabelKey, cChildrenKey, cIconKey, cIDKey, cSelectedID, cBadgeKey, C.uint64_t(cbID))
	return renderer.ViewHandle(uintptr(ptr))
}

func updateOutlineView(handle renderer.ViewHandle, node *renderer.RenderNode) {
	cData := C.CString(node.Props.OutlineData)
	defer C.free(unsafe.Pointer(cData))
	cSelectedID := C.CString(node.Props.SelectedID)
	defer C.free(unsafe.Pointer(cSelectedID))

	C.JVUpdateOutlineView(unsafe.Pointer(handle), cData, cSelectedID)
}

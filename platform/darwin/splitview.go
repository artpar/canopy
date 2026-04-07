package darwin

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework Cocoa -framework QuartzCore

#include <stdlib.h>
#include "splitview.h"
*/
import "C"
import (
	"canopy/renderer"
	"unsafe"
)

func createSplitView(node *renderer.RenderNode) renderer.ViewHandle {
	cDividerStyle := C.CString(node.Props.DividerStyle)
	defer C.free(unsafe.Pointer(cDividerStyle))

	ptr := C.JVCreateSplitView(cDividerStyle, C.bool(node.Props.Vertical))
	return renderer.ViewHandle(uintptr(ptr))
}

func updateSplitView(handle renderer.ViewHandle, node *renderer.RenderNode) {
	cDividerStyle := C.CString(node.Props.DividerStyle)
	defer C.free(unsafe.Pointer(cDividerStyle))

	C.JVUpdateSplitView(unsafe.Pointer(handle), cDividerStyle, C.bool(node.Props.Vertical), C.int(node.Props.CollapsedPane))
}

func setSplitViewChildren(parentHandle renderer.ViewHandle, childHandles []renderer.ViewHandle) {
	if len(childHandles) == 0 {
		C.JVSplitViewSetChildren(unsafe.Pointer(parentHandle), nil, 0)
		return
	}

	ptrs := make([]unsafe.Pointer, len(childHandles))
	for i, h := range childHandles {
		ptrs[i] = unsafe.Pointer(h)
	}

	C.JVSplitViewSetChildren(unsafe.Pointer(parentHandle), &ptrs[0], C.int(len(ptrs)))
}

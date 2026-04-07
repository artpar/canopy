package darwin

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework Cocoa

#include <stdlib.h>
#include "list.h"
*/
import "C"
import (
	"canopy/renderer"
	"unsafe"
)

func createListView(node *renderer.RenderNode) renderer.ViewHandle {
	cJustify := C.CString(node.Props.Justify)
	defer C.free(unsafe.Pointer(cJustify))
	cAlign := C.CString(node.Props.Align)
	defer C.free(unsafe.Pointer(cAlign))

	ptr := C.JVCreateList(cJustify, cAlign, C.int(node.Props.Gap), C.int(node.Props.Padding))
	return renderer.ViewHandle(uintptr(ptr))
}

func updateListView(handle renderer.ViewHandle, node *renderer.RenderNode) {
	cJustify := C.CString(node.Props.Justify)
	defer C.free(unsafe.Pointer(cJustify))
	cAlign := C.CString(node.Props.Align)
	defer C.free(unsafe.Pointer(cAlign))

	C.JVUpdateList(unsafe.Pointer(handle), cJustify, cAlign, C.int(node.Props.Gap), C.int(node.Props.Padding))
}

func setListChildren(parentHandle renderer.ViewHandle, childHandles []renderer.ViewHandle) {
	if len(childHandles) == 0 {
		C.JVListSetChildren(unsafe.Pointer(parentHandle), nil, 0)
		return
	}

	ptrs := make([]unsafe.Pointer, len(childHandles))
	for i, h := range childHandles {
		ptrs[i] = unsafe.Pointer(h)
	}

	C.JVListSetChildren(unsafe.Pointer(parentHandle), &ptrs[0], C.int(len(ptrs)))
}

package darwin

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework Cocoa

#include <stdlib.h>
#include "stackview.h"
*/
import "C"
import (
	"canopy/renderer"
	"unsafe"
)

func createStackView(node *renderer.RenderNode, horizontal bool) renderer.ViewHandle {
	cJustify := C.CString(node.Props.Justify)
	defer C.free(unsafe.Pointer(cJustify))
	cAlign := C.CString(node.Props.Align)
	defer C.free(unsafe.Pointer(cAlign))

	ptr := C.JVCreateStackView(C.bool(horizontal), cJustify, cAlign, C.int(node.Props.Gap), C.int(node.Props.Padding))
	return renderer.ViewHandle(uintptr(ptr))
}

func updateStackView(handle renderer.ViewHandle, node *renderer.RenderNode, horizontal bool) {
	cJustify := C.CString(node.Props.Justify)
	defer C.free(unsafe.Pointer(cJustify))
	cAlign := C.CString(node.Props.Align)
	defer C.free(unsafe.Pointer(cAlign))

	C.JVUpdateStackView(unsafe.Pointer(handle), cJustify, cAlign, C.int(node.Props.Gap), C.int(node.Props.Padding))
}

func setStackViewChildren(parentHandle renderer.ViewHandle, childHandles []renderer.ViewHandle) {
	if len(childHandles) == 0 {
		C.JVStackViewSetChildren(unsafe.Pointer(parentHandle), nil, 0)
		return
	}

	ptrs := make([]unsafe.Pointer, len(childHandles))
	for i, h := range childHandles {
		ptrs[i] = unsafe.Pointer(h)
	}

	C.JVStackViewSetChildren(unsafe.Pointer(parentHandle), &ptrs[0], C.int(len(ptrs)))
}

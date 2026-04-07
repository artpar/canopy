package darwin

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework Cocoa

#include <stdlib.h>
#include "card.h"
*/
import "C"
import (
	"canopy/renderer"
	"unsafe"
)

func createCardView(node *renderer.RenderNode) renderer.ViewHandle {
	cTitle := C.CString(node.Props.Title)
	defer C.free(unsafe.Pointer(cTitle))
	cSubtitle := C.CString(node.Props.Subtitle)
	defer C.free(unsafe.Pointer(cSubtitle))

	padding := node.Props.Padding
	if padding == 0 {
		padding = 16
	}

	ptr := C.JVCreateCard(cTitle, cSubtitle, C.int(padding))
	return renderer.ViewHandle(uintptr(ptr))
}

func updateCardView(handle renderer.ViewHandle, node *renderer.RenderNode) {
	cTitle := C.CString(node.Props.Title)
	defer C.free(unsafe.Pointer(cTitle))
	cSubtitle := C.CString(node.Props.Subtitle)
	defer C.free(unsafe.Pointer(cSubtitle))

	padding := node.Props.Padding
	if padding == 0 {
		padding = 16
	}

	C.JVUpdateCard(unsafe.Pointer(handle), cTitle, cSubtitle, C.int(padding))
}

func setCardChildren(parentHandle renderer.ViewHandle, childHandles []renderer.ViewHandle) {
	if len(childHandles) == 0 {
		C.JVCardSetChildren(unsafe.Pointer(parentHandle), nil, 0)
		return
	}

	ptrs := make([]unsafe.Pointer, len(childHandles))
	for i, h := range childHandles {
		ptrs[i] = unsafe.Pointer(h)
	}

	C.JVCardSetChildren(unsafe.Pointer(parentHandle), &ptrs[0], C.int(len(ptrs)))
}

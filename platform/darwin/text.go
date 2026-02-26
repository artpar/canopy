package darwin

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework Cocoa

#include <stdlib.h>
#include "text.h"
*/
import "C"
import (
	"jview/renderer"
	"unsafe"
)

func createTextView(node *renderer.RenderNode) renderer.ViewHandle {
	cContent := C.CString(node.Props.Content)
	defer C.free(unsafe.Pointer(cContent))
	cVariant := C.CString(node.Props.Variant)
	defer C.free(unsafe.Pointer(cVariant))

	ptr := C.JVCreateText(cContent, cVariant, C.int(node.Props.MaxLines))
	return renderer.ViewHandle(uintptr(ptr))
}

func updateTextView(handle renderer.ViewHandle, node *renderer.RenderNode) {
	cContent := C.CString(node.Props.Content)
	defer C.free(unsafe.Pointer(cContent))
	cVariant := C.CString(node.Props.Variant)
	defer C.free(unsafe.Pointer(cVariant))

	C.JVUpdateText(unsafe.Pointer(handle), cContent, cVariant, C.int(node.Props.MaxLines))
}

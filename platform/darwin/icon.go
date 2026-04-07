package darwin

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework Cocoa

#include <stdlib.h>
#include "icon.h"
*/
import "C"
import (
	"canopy/renderer"
	"unsafe"
)

func createIconView(node *renderer.RenderNode) renderer.ViewHandle {
	cName := C.CString(node.Props.Name)
	defer C.free(unsafe.Pointer(cName))

	ptr := C.JVCreateIcon(cName, C.int(node.Props.Size))
	return renderer.ViewHandle(uintptr(ptr))
}

func updateIconView(handle renderer.ViewHandle, node *renderer.RenderNode) {
	cName := C.CString(node.Props.Name)
	defer C.free(unsafe.Pointer(cName))

	C.JVUpdateIcon(unsafe.Pointer(handle), cName, C.int(node.Props.Size))
}

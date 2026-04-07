package darwin

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework Cocoa

#include "divider.h"
*/
import "C"
import (
	"canopy/renderer"
	"unsafe"
)

func createDividerView(node *renderer.RenderNode) renderer.ViewHandle {
	ptr := C.JVCreateDivider()
	return renderer.ViewHandle(uintptr(ptr))
}

func updateDividerView(handle renderer.ViewHandle, node *renderer.RenderNode) {
	C.JVUpdateDivider(unsafe.Pointer(handle))
}

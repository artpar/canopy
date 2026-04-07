package darwin

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework Cocoa

#include <stdlib.h>
#include "image.h"
*/
import "C"
import (
	"canopy/renderer"
	"unsafe"
)

func createImageView(node *renderer.RenderNode) renderer.ViewHandle {
	cSrc := C.CString(node.Props.Src)
	defer C.free(unsafe.Pointer(cSrc))
	cAlt := C.CString(node.Props.Alt)
	defer C.free(unsafe.Pointer(cAlt))

	ptr := C.JVCreateImage(cSrc, cAlt, C.int(node.Props.Width), C.int(node.Props.Height))
	return renderer.ViewHandle(uintptr(ptr))
}

func updateImageView(handle renderer.ViewHandle, node *renderer.RenderNode) {
	cSrc := C.CString(node.Props.Src)
	defer C.free(unsafe.Pointer(cSrc))
	cAlt := C.CString(node.Props.Alt)
	defer C.free(unsafe.Pointer(cAlt))

	C.JVUpdateImage(unsafe.Pointer(handle), cSrc, cAlt, C.int(node.Props.Width), C.int(node.Props.Height))
}

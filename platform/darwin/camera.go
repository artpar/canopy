package darwin

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework Cocoa -framework AVFoundation -framework CoreMedia

#include <stdlib.h>
#include <stdbool.h>
#include "camera.h"
*/
import "C"
import (
	"canopy/renderer"
	"unsafe"
)

func createCameraView(node *renderer.RenderNode, surfaceID string) renderer.ViewHandle {
	cPos := C.CString(node.Props.DevicePosition)
	defer C.free(unsafe.Pointer(cPos))

	var captureCbID, errorCbID uint64
	if id, ok := node.Callbacks["capture"]; ok {
		captureCbID = uint64(id)
	}
	if id, ok := node.Callbacks["error"]; ok {
		errorCbID = uint64(id)
	}

	ptr := C.JVCreateCamera(cPos,
		C.bool(node.Props.Mirrored),
		C.uint64_t(captureCbID), C.uint64_t(errorCbID))
	return renderer.ViewHandle(uintptr(ptr))
}

func updateCameraView(handle renderer.ViewHandle, node *renderer.RenderNode) {
	cPos := C.CString(node.Props.DevicePosition)
	defer C.free(unsafe.Pointer(cPos))

	C.JVUpdateCamera(unsafe.Pointer(handle), cPos,
		C.bool(node.Props.Mirrored))
}

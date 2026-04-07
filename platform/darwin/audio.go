package darwin

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework Cocoa -framework AVFoundation -framework CoreMedia

#include <stdlib.h>
#include <stdbool.h>
#include "audio.h"
*/
import "C"
import (
	"canopy/renderer"
	"unsafe"
)

func createAudioView(node *renderer.RenderNode, surfaceID string) renderer.ViewHandle {
	cSrc := C.CString(node.Props.Src)
	defer C.free(unsafe.Pointer(cSrc))

	var cbID uint64
	if id, ok := node.Callbacks["ended"]; ok {
		cbID = uint64(id)
	}

	ptr := C.JVCreateAudio(cSrc,
		C.bool(node.Props.Autoplay), C.bool(node.Props.Loop),
		C.uint64_t(cbID))
	return renderer.ViewHandle(uintptr(ptr))
}

func updateAudioView(handle renderer.ViewHandle, node *renderer.RenderNode) {
	cSrc := C.CString(node.Props.Src)
	defer C.free(unsafe.Pointer(cSrc))

	C.JVUpdateAudio(unsafe.Pointer(handle), cSrc, C.bool(node.Props.Loop))
}

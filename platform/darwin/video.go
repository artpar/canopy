package darwin

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework Cocoa -framework AVKit -framework AVFoundation -framework CoreMedia

#include <stdlib.h>
#include <stdbool.h>
#include "video.h"
*/
import "C"
import (
	"canopy/renderer"
	"unsafe"
)

func createVideoView(node *renderer.RenderNode, surfaceID string) renderer.ViewHandle {
	cSrc := C.CString(node.Props.Src)
	defer C.free(unsafe.Pointer(cSrc))

	var cbID uint64
	if id, ok := node.Callbacks["ended"]; ok {
		cbID = uint64(id)
	}

	ptr := C.JVCreateVideo(cSrc,
		C.int(node.Props.Width), C.int(node.Props.Height),
		C.bool(node.Props.Autoplay), C.bool(node.Props.Loop),
		C.bool(node.Props.Controls), C.bool(node.Props.Muted),
		C.uint64_t(cbID))
	return renderer.ViewHandle(uintptr(ptr))
}

func updateVideoView(handle renderer.ViewHandle, node *renderer.RenderNode) {
	cSrc := C.CString(node.Props.Src)
	defer C.free(unsafe.Pointer(cSrc))

	C.JVUpdateVideo(unsafe.Pointer(handle), cSrc,
		C.int(node.Props.Width), C.int(node.Props.Height),
		C.bool(node.Props.Loop), C.bool(node.Props.Controls),
		C.bool(node.Props.Muted))
}

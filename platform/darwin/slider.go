package darwin

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework Cocoa

#include <stdlib.h>
#include "slider.h"
*/
import "C"
import (
	"canopy/renderer"
	"unsafe"
)

func createSliderView(node *renderer.RenderNode, surfaceID string) renderer.ViewHandle {
	var cbID uint64
	if id, ok := node.Callbacks["slide"]; ok {
		cbID = uint64(id)
	}

	ptr := C.JVCreateSlider(
		C.double(node.Props.Min),
		C.double(node.Props.Max),
		C.double(node.Props.Step),
		C.double(node.Props.SliderValue),
		C.uint64_t(cbID),
	)
	return renderer.ViewHandle(uintptr(ptr))
}

func updateSliderView(handle renderer.ViewHandle, node *renderer.RenderNode) {
	C.JVUpdateSlider(
		unsafe.Pointer(handle),
		C.double(node.Props.Min),
		C.double(node.Props.Max),
		C.double(node.Props.Step),
		C.double(node.Props.SliderValue),
	)
}

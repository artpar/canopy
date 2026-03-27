package darwin

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework Cocoa

#include "progressbar.h"
*/
import "C"
import (
	"jview/renderer"
	"unsafe"
)

func createProgressBarView(node *renderer.RenderNode) renderer.ViewHandle {
	ptr := C.JVCreateProgressBar(
		C.double(node.Props.Min),
		C.double(node.Props.Max),
		C.double(node.Props.ProgressValue),
		C.bool(node.Props.Indeterminate),
	)
	return renderer.ViewHandle(uintptr(ptr))
}

func updateProgressBarView(handle renderer.ViewHandle, node *renderer.RenderNode) {
	C.JVUpdateProgressBar(
		unsafe.Pointer(handle),
		C.double(node.Props.Min),
		C.double(node.Props.Max),
		C.double(node.Props.ProgressValue),
		C.bool(node.Props.Indeterminate),
	)
}

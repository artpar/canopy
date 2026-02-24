package darwin

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework Cocoa

#include <stdlib.h>
#include "style.h"
*/
import "C"
import (
	"jview/protocol"
	"jview/renderer"
	"unsafe"
)

func applyStyle(handle renderer.ViewHandle, style protocol.StyleProps) {
	if style == (protocol.StyleProps{}) {
		return
	}

	cBg := C.CString(style.BackgroundColor)
	defer C.free(unsafe.Pointer(cBg))
	cTc := C.CString(style.TextColor)
	defer C.free(unsafe.Pointer(cTc))
	cFw := C.CString(style.FontWeight)
	defer C.free(unsafe.Pointer(cFw))
	cTa := C.CString(style.TextAlign)
	defer C.free(unsafe.Pointer(cTa))

	C.JVApplyStyle(
		unsafe.Pointer(handle),
		cBg, cTc,
		C.double(style.CornerRadius),
		C.double(style.Width),
		C.double(style.Height),
		C.double(style.FontSize),
		cFw, cTa,
		C.double(style.Opacity),
	)
}

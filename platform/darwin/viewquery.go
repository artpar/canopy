package darwin

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework Cocoa

#include "viewquery.h"
#include <stdlib.h>
*/
import "C"
import (
	"jview/renderer"
	"unsafe"
)

// QueryLayout returns the computed frame of a view in window coordinates.
func (r *DarwinRenderer) QueryLayout(surfaceID string, componentID string) renderer.LayoutInfo {
	handle := r.GetHandle(surfaceID, componentID)
	if handle == 0 {
		return renderer.LayoutInfo{}
	}

	frame := C.JVGetViewFrame(unsafe.Pointer(handle))
	return renderer.LayoutInfo{
		X:      float64(frame.x),
		Y:      float64(frame.y),
		Width:  float64(frame.width),
		Height: float64(frame.height),
	}
}

// QueryStyle returns the computed style properties of a view.
func (r *DarwinRenderer) QueryStyle(surfaceID string, componentID string) renderer.StyleInfo {
	handle := r.GetHandle(surfaceID, componentID)
	if handle == 0 {
		return renderer.StyleInfo{}
	}

	style := C.JVGetViewStyle(unsafe.Pointer(handle))
	defer C.JVFreeViewStyle(style)

	info := renderer.StyleInfo{
		FontSize: float64(style.fontSize),
		Bold:     style.bold != 0,
		Italic:   style.italic != 0,
		Hidden:   style.hidden != 0,
		Opacity:  float64(style.opacity),
	}

	if style.fontName != nil {
		info.FontName = C.GoString(style.fontName)
	}
	if style.textColor != nil {
		info.TextColor = C.GoString(style.textColor)
	}
	if style.bgColor != nil {
		info.BgColor = C.GoString(style.bgColor)
	}

	return info
}

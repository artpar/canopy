package darwin

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework Cocoa

#include <stdlib.h>
#include <stdint.h>
#include "richtexteditor.h"
*/
import "C"
import (
	"jview/renderer"
	"unsafe"
)

func createRichTextEditorView(node *renderer.RenderNode, surfaceID string) renderer.ViewHandle {
	cContent := C.CString(node.Props.RichContent)
	defer C.free(unsafe.Pointer(cContent))

	var cbID uint64
	if id, ok := node.Callbacks["change"]; ok {
		cbID = uint64(id)
	}

	ptr := C.JVCreateRichTextEditor(cContent, C.bool(node.Props.Editable), C.uint64_t(cbID))

	// Set format callback ID if present
	if fmtID, ok := node.Callbacks["formatchange"]; ok && fmtID != 0 {
		C.JVRichTextEditorSetFormatCallbackID(ptr, C.uint64_t(fmtID))
	}

	return renderer.ViewHandle(uintptr(ptr))
}

func updateRichTextEditorView(handle renderer.ViewHandle, node *renderer.RenderNode) {
	cContent := C.CString(node.Props.RichContent)
	defer C.free(unsafe.Pointer(cContent))

	C.JVUpdateRichTextEditor(unsafe.Pointer(handle), cContent, C.bool(node.Props.Editable))

	// Sync format callback ID (may change on forEach re-expansion)
	if fmtID, ok := node.Callbacks["formatchange"]; ok {
		C.JVRichTextEditorSetFormatCallbackID(unsafe.Pointer(handle), C.uint64_t(fmtID))
	}
}

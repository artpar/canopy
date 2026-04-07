package darwin

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework Cocoa

#include <stdlib.h>
#include "textfield.h"
*/
import "C"
import (
	"canopy/renderer"
	"unsafe"
)

func createTextFieldView(node *renderer.RenderNode, surfaceID string) renderer.ViewHandle {
	cPlaceholder := C.CString(node.Props.Placeholder)
	defer C.free(unsafe.Pointer(cPlaceholder))
	cValue := C.CString(node.Props.Value)
	defer C.free(unsafe.Pointer(cValue))
	cInputType := C.CString(node.Props.InputType)
	defer C.free(unsafe.Pointer(cInputType))

	var cbID uint64
	if id, ok := node.Callbacks["change"]; ok {
		cbID = uint64(id)
	}

	ptr := C.JVCreateTextField(cPlaceholder, cValue, cInputType, C.bool(node.Props.ReadOnly), C.uint64_t(cbID))
	return renderer.ViewHandle(uintptr(ptr))
}

func updateTextFieldView(handle renderer.ViewHandle, node *renderer.RenderNode) {
	cPlaceholder := C.CString(node.Props.Placeholder)
	defer C.free(unsafe.Pointer(cPlaceholder))
	cValue := C.CString(node.Props.Value)
	defer C.free(unsafe.Pointer(cValue))
	cInputType := C.CString(node.Props.InputType)
	defer C.free(unsafe.Pointer(cInputType))

	C.JVUpdateTextField(unsafe.Pointer(handle), cPlaceholder, cValue, cInputType, C.bool(node.Props.ReadOnly))

	// Update validation errors
	setTextFieldErrors(handle, node.Props.ValidationErrors)
}

func setTextFieldErrors(handle renderer.ViewHandle, errors []string) {
	n := len(errors)
	if n == 0 {
		C.JVSetTextFieldErrors(unsafe.Pointer(handle), nil, 0)
		return
	}

	cErrors := make([]*C.char, n)
	for i, e := range errors {
		cErrors[i] = C.CString(e)
	}
	C.JVSetTextFieldErrors(unsafe.Pointer(handle), &cErrors[0], C.int(n))
	for _, ce := range cErrors {
		C.free(unsafe.Pointer(ce))
	}
}

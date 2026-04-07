package darwin

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework Cocoa

#include <stdlib.h>
#include "choicepicker.h"
*/
import "C"
import (
	"canopy/renderer"
	"unsafe"
)

func createChoicePickerView(node *renderer.RenderNode, surfaceID string) renderer.ViewHandle {
	labels, values, count := optionsToCArrays(node.Props.Options)
	defer freeOptionArrays(labels, values, count)

	selected := ""
	if len(node.Props.Selected) > 0 {
		selected = node.Props.Selected[0]
	}
	cSelected := C.CString(selected)
	defer C.free(unsafe.Pointer(cSelected))

	var cbID uint64
	if id, ok := node.Callbacks["select"]; ok {
		cbID = uint64(id)
	}

	ptr := C.JVCreateChoicePicker(labels, values, C.int(count), cSelected, C.uint64_t(cbID))
	return renderer.ViewHandle(uintptr(ptr))
}

func updateChoicePickerView(handle renderer.ViewHandle, node *renderer.RenderNode) {
	labels, values, count := optionsToCArrays(node.Props.Options)
	defer freeOptionArrays(labels, values, count)

	selected := ""
	if len(node.Props.Selected) > 0 {
		selected = node.Props.Selected[0]
	}
	cSelected := C.CString(selected)
	defer C.free(unsafe.Pointer(cSelected))

	C.JVUpdateChoicePicker(unsafe.Pointer(handle), labels, values, C.int(count), cSelected)
}

func optionsToCArrays(opts []renderer.OptionItem) (**C.char, **C.char, int) {
	n := len(opts)
	if n == 0 {
		return nil, nil, 0
	}
	labelsArr := C.malloc(C.size_t(n) * C.size_t(unsafe.Sizeof((*C.char)(nil))))
	valuesArr := C.malloc(C.size_t(n) * C.size_t(unsafe.Sizeof((*C.char)(nil))))
	labels := (**C.char)(labelsArr)
	values := (**C.char)(valuesArr)

	labelSlice := unsafe.Slice(labels, n)
	valueSlice := unsafe.Slice(values, n)
	for i, o := range opts {
		labelSlice[i] = C.CString(o.Label)
		valueSlice[i] = C.CString(o.Value)
	}
	return labels, values, n
}

func freeOptionArrays(labels, values **C.char, count int) {
	if count == 0 {
		return
	}
	labelSlice := unsafe.Slice(labels, count)
	valueSlice := unsafe.Slice(values, count)
	for i := 0; i < count; i++ {
		C.free(unsafe.Pointer(labelSlice[i]))
		C.free(unsafe.Pointer(valueSlice[i]))
	}
	C.free(unsafe.Pointer(labels))
	C.free(unsafe.Pointer(values))
}

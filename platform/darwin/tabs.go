package darwin

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework Cocoa

#include <stdlib.h>
#include "tabs.h"
*/
import "C"
import (
	"canopy/renderer"
	"unsafe"
)

func createTabsView(node *renderer.RenderNode, surfaceID string) renderer.ViewHandle {
	labels, count := tabLabelsToCArray(node.Props.TabLabels)
	defer freeTabLabels(labels, count)

	cActiveTab := C.CString(node.Props.ActiveTab)
	defer C.free(unsafe.Pointer(cActiveTab))

	var cbID uint64
	if id, ok := node.Callbacks["select"]; ok {
		cbID = uint64(id)
	}

	ptr := C.JVCreateTabs(labels, C.int(count), cActiveTab, C.uint64_t(cbID))
	handle := renderer.ViewHandle(uintptr(ptr))

	// Store child IDs for tab selection callbacks
	setTabsChildIDs(handle, node.ChildIDs)

	return handle
}

func updateTabsView(handle renderer.ViewHandle, node *renderer.RenderNode) {
	labels, count := tabLabelsToCArray(node.Props.TabLabels)
	defer freeTabLabels(labels, count)

	cActiveTab := C.CString(node.Props.ActiveTab)
	defer C.free(unsafe.Pointer(cActiveTab))

	// Update child IDs before updating tabs (so active tab selection can use them)
	setTabsChildIDs(handle, node.ChildIDs)

	C.JVUpdateTabs(unsafe.Pointer(handle), labels, C.int(count), cActiveTab)
}

func setTabsChildren(parentHandle renderer.ViewHandle, childHandles []renderer.ViewHandle) {
	if len(childHandles) == 0 {
		C.JVTabsSetChildren(unsafe.Pointer(parentHandle), nil, 0)
		return
	}

	ptrs := make([]unsafe.Pointer, len(childHandles))
	for i, h := range childHandles {
		ptrs[i] = unsafe.Pointer(h)
	}

	C.JVTabsSetChildren(unsafe.Pointer(parentHandle), &ptrs[0], C.int(len(ptrs)))
}

func setTabsChildIDs(handle renderer.ViewHandle, childIDs []string) {
	if len(childIDs) == 0 {
		return
	}
	cIDs, count := tabLabelsToCArray(childIDs)
	defer freeTabLabels(cIDs, count)
	C.JVTabsSetChildIDs(unsafe.Pointer(handle), cIDs, C.int(count))
}

func tabLabelsToCArray(labels []string) (**C.char, int) {
	n := len(labels)
	if n == 0 {
		return nil, 0
	}
	arr := C.malloc(C.size_t(n) * C.size_t(unsafe.Sizeof((*C.char)(nil))))
	cLabels := (**C.char)(arr)
	slice := unsafe.Slice(cLabels, n)
	for i, l := range labels {
		slice[i] = C.CString(l)
	}
	return cLabels, n
}

func freeTabLabels(labels **C.char, count int) {
	if count == 0 {
		return
	}
	slice := unsafe.Slice(labels, count)
	for i := 0; i < count; i++ {
		C.free(unsafe.Pointer(slice[i]))
	}
	C.free(unsafe.Pointer(labels))
}

package darwin

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework Cocoa

#include <stdlib.h>
#include "app.h"
*/
import "C"
import (
	"jview/protocol"
	"jview/renderer"
	"log"
	"sync"
	"unsafe"
)

// DarwinRenderer implements renderer.Renderer for macOS using AppKit.
type DarwinRenderer struct {
	mu sync.Mutex
	// surfaceID → componentID → ViewHandle
	handles map[string]map[string]renderer.ViewHandle
	// ViewHandle → ComponentType
	types map[renderer.ViewHandle]protocol.ComponentType
	// surfaceID → componentID → eventType → CallbackID
	callbacks map[string]map[string]map[string]renderer.CallbackID
}

func NewRenderer() *DarwinRenderer {
	return &DarwinRenderer{
		handles:   make(map[string]map[string]renderer.ViewHandle),
		types:     make(map[renderer.ViewHandle]protocol.ComponentType),
		callbacks: make(map[string]map[string]map[string]renderer.CallbackID),
	}
}

func (r *DarwinRenderer) CreateWindow(spec renderer.WindowSpec) {
	cTitle := C.CString(spec.Title)
	defer C.free(unsafe.Pointer(cTitle))
	cSID := C.CString(spec.SurfaceID)
	defer C.free(unsafe.Pointer(cSID))

	C.JVCreateWindow(cTitle, C.int(spec.Width), C.int(spec.Height), cSID)
}

func (r *DarwinRenderer) DestroyWindow(surfaceID string) {
	cSID := C.CString(surfaceID)
	defer C.free(unsafe.Pointer(cSID))
	C.JVDestroyWindow(cSID)

	r.mu.Lock()
	delete(r.handles, surfaceID)
	delete(r.callbacks, surfaceID)
	r.mu.Unlock()
}

func (r *DarwinRenderer) CreateView(surfaceID string, node *renderer.RenderNode) renderer.ViewHandle {
	var handle renderer.ViewHandle

	switch node.Type {
	case protocol.CompText:
		handle = createTextView(node)
	case protocol.CompRow:
		handle = createStackView(node, true)
	case protocol.CompColumn:
		handle = createStackView(node, false)
	case protocol.CompCard:
		handle = createCardView(node)
	case protocol.CompButton:
		handle = createButtonView(node, surfaceID)
	case protocol.CompTextField:
		handle = createTextFieldView(node, surfaceID)
	case protocol.CompCheckBox:
		handle = createCheckBoxView(node, surfaceID)
	case protocol.CompDivider:
		handle = createDividerView(node)
	case protocol.CompIcon:
		handle = createIconView(node)
	case protocol.CompImage:
		handle = createImageView(node)
	case protocol.CompSlider:
		handle = createSliderView(node, surfaceID)
	case protocol.CompChoicePicker:
		handle = createChoicePickerView(node, surfaceID)
	case protocol.CompDateTimeInput:
		handle = createDateTimeInputView(node, surfaceID)
	case protocol.CompList:
		handle = createListView(node)
	default:
		log.Printf("darwin: unsupported component type %s", node.Type)
		return 0
	}

	r.mu.Lock()
	if r.handles[surfaceID] == nil {
		r.handles[surfaceID] = make(map[string]renderer.ViewHandle)
	}
	r.handles[surfaceID][node.ComponentID] = handle
	r.types[handle] = node.Type
	r.mu.Unlock()

	return handle
}

func (r *DarwinRenderer) UpdateView(surfaceID string, handle renderer.ViewHandle, node *renderer.RenderNode) {
	switch node.Type {
	case protocol.CompText:
		updateTextView(handle, node)
	case protocol.CompRow:
		updateStackView(handle, node, true)
	case protocol.CompColumn:
		updateStackView(handle, node, false)
	case protocol.CompCard:
		updateCardView(handle, node)
	case protocol.CompButton:
		updateButtonView(handle, node)
	case protocol.CompTextField:
		updateTextFieldView(handle, node)
	case protocol.CompCheckBox:
		updateCheckBoxView(handle, node)
	case protocol.CompDivider:
		updateDividerView(handle, node)
	case protocol.CompIcon:
		updateIconView(handle, node)
	case protocol.CompImage:
		updateImageView(handle, node)
	case protocol.CompSlider:
		updateSliderView(handle, node)
	case protocol.CompChoicePicker:
		updateChoicePickerView(handle, node)
	case protocol.CompDateTimeInput:
		updateDateTimeInputView(handle, node)
	case protocol.CompList:
		updateListView(handle, node)
	default:
		log.Printf("darwin: unsupported update for component type %s", node.Type)
	}
}

func (r *DarwinRenderer) SetChildren(surfaceID string, parentHandle renderer.ViewHandle, childHandles []renderer.ViewHandle) {
	parentType := r.GetComponentType(parentHandle)

	switch parentType {
	case protocol.CompRow, protocol.CompColumn:
		setStackViewChildren(parentHandle, childHandles)
	case protocol.CompCard:
		setCardChildren(parentHandle, childHandles)
	case protocol.CompList:
		setListChildren(parentHandle, childHandles)
	default:
		log.Printf("darwin: SetChildren not supported for type %s", parentType)
	}
}

func (r *DarwinRenderer) RemoveView(surfaceID string, handle renderer.ViewHandle) {
	removeView(handle)
	r.mu.Lock()
	delete(r.types, handle)
	r.mu.Unlock()
}

func (r *DarwinRenderer) GetHandle(surfaceID string, componentID string) renderer.ViewHandle {
	r.mu.Lock()
	defer r.mu.Unlock()
	if m, ok := r.handles[surfaceID]; ok {
		return m[componentID]
	}
	return 0
}

func (r *DarwinRenderer) RegisterCallback(surfaceID string, componentID string, eventType string, fn func(string)) renderer.CallbackID {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.callbacks[surfaceID] == nil {
		r.callbacks[surfaceID] = make(map[string]map[string]renderer.CallbackID)
	}
	if r.callbacks[surfaceID][componentID] == nil {
		r.callbacks[surfaceID][componentID] = make(map[string]renderer.CallbackID)
	}

	// If there's already a callback for this event, unregister the old one
	if oldID, exists := r.callbacks[surfaceID][componentID][eventType]; exists {
		globalRegistry.Unregister(uint64(oldID))
	}

	id := globalRegistry.Register(fn)
	r.callbacks[surfaceID][componentID][eventType] = renderer.CallbackID(id)
	return renderer.CallbackID(id)
}

func (r *DarwinRenderer) UnregisterCallback(id renderer.CallbackID) {
	globalRegistry.Unregister(uint64(id))
}

func (r *DarwinRenderer) SetRootView(surfaceID string, handle renderer.ViewHandle) {
	cSID := C.CString(surfaceID)
	defer C.free(unsafe.Pointer(cSID))
	C.JVSetWindowRootView(cSID, unsafe.Pointer(handle))
}

func (r *DarwinRenderer) GetComponentType(handle renderer.ViewHandle) protocol.ComponentType {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.types[handle]
}

// InvokeCallback programmatically triggers a registered callback for testing.
func (r *DarwinRenderer) InvokeCallback(surfaceID, componentID, eventType, data string) {
	r.mu.Lock()
	var cbID renderer.CallbackID
	if s, ok := r.callbacks[surfaceID]; ok {
		if c, ok := s[componentID]; ok {
			cbID = c[eventType]
		}
	}
	r.mu.Unlock()
	if cbID != 0 {
		globalRegistry.Invoke(uint64(cbID), data)
	}
}

// removeView removes an NSView from its superview.
func removeView(handle renderer.ViewHandle) {
	// Implemented in ObjC — for now just a placeholder
	// In Phase 2 we'll add proper cleanup
}

package darwin

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework Cocoa

#include <stdlib.h>
#include "app.h"
*/
import "C"
import (
	"jview/jlog"
	"jview/protocol"
	"jview/renderer"
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
	// surfaceID → padding
	surfacePadding map[string]int
}

func NewRenderer() *DarwinRenderer {
	return &DarwinRenderer{
		handles:        make(map[string]map[string]renderer.ViewHandle),
		types:          make(map[renderer.ViewHandle]protocol.ComponentType),
		callbacks:      make(map[string]map[string]map[string]renderer.CallbackID),
		surfacePadding: make(map[string]int),
	}
}

func (r *DarwinRenderer) CreateWindow(spec renderer.WindowSpec) {
	cTitle := C.CString(spec.Title)
	defer C.free(unsafe.Pointer(cTitle))
	cSID := C.CString(spec.SurfaceID)
	defer C.free(unsafe.Pointer(cSID))
	cBg := C.CString(spec.BackgroundColor)
	defer C.free(unsafe.Pointer(cBg))

	C.JVCreateWindow(cTitle, C.int(spec.Width), C.int(spec.Height), cSID, cBg)

	r.mu.Lock()
	r.surfacePadding[spec.SurfaceID] = spec.Padding
	r.mu.Unlock()
}

func (r *DarwinRenderer) DestroyWindow(surfaceID string) {
	cSID := C.CString(surfaceID)
	defer C.free(unsafe.Pointer(cSID))
	C.JVDestroyWindow(cSID)

	r.mu.Lock()
	// Unregister all callbacks from globalRegistry before removing tracking
	if comps, ok := r.callbacks[surfaceID]; ok {
		for _, events := range comps {
			for _, cbID := range events {
				globalRegistry.Unregister(uint64(cbID))
			}
		}
	}
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
	case protocol.CompTabs:
		handle = createTabsView(node, surfaceID)
	case protocol.CompModal:
		handle = createModalView(node, surfaceID)
	case protocol.CompVideo:
		handle = createVideoView(node, surfaceID)
	case protocol.CompAudioPlayer:
		handle = createAudioView(node, surfaceID)
	case protocol.CompSplitView:
		handle = createSplitView(node)
	case protocol.CompSearchField:
		handle = createSearchFieldView(node, surfaceID)
	case protocol.CompOutlineView:
		handle = createOutlineView(node, surfaceID)
	case protocol.CompRichTextEditor:
		handle = createRichTextEditorView(node, surfaceID)
	default:
		jlog.Warnf("darwin", surfaceID, "unsupported component type %s", node.Type)
		return 0
	}

	applyStyle(handle, node.Style)

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
	case protocol.CompTabs:
		updateTabsView(handle, node)
	case protocol.CompModal:
		updateModalView(handle, node)
	case protocol.CompVideo:
		updateVideoView(handle, node)
	case protocol.CompAudioPlayer:
		updateAudioView(handle, node)
	case protocol.CompSplitView:
		updateSplitView(handle, node)
	case protocol.CompSearchField:
		updateSearchFieldView(handle, node)
	case protocol.CompOutlineView:
		updateOutlineView(handle, node)
	case protocol.CompRichTextEditor:
		updateRichTextEditorView(handle, node)
	default:
		jlog.Warnf("darwin", surfaceID, "unsupported update for component type %s", node.Type)
	}

	applyStyle(handle, node.Style)
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
	case protocol.CompTabs:
		setTabsChildren(parentHandle, childHandles)
	case protocol.CompModal:
		setModalChildren(parentHandle, childHandles)
	case protocol.CompSplitView:
		setSplitViewChildren(parentHandle, childHandles)
	default:
		jlog.Warnf("darwin", surfaceID, "SetChildren not supported for type %s", parentType)
	}
}

func (r *DarwinRenderer) RemoveView(surfaceID string, componentID string, handle renderer.ViewHandle) {
	// Type-specific cleanup (observer removal, player pause, etc.) before removing from superview
	r.mu.Lock()
	compType := r.types[handle]
	r.mu.Unlock()
	cleanupView(handle, compType)

	removeView(handle)

	r.mu.Lock()
	defer r.mu.Unlock()

	// Clean up handle map
	if m, ok := r.handles[surfaceID]; ok {
		delete(m, componentID)
	}

	// Clean up callbacks for this component + unregister from globalRegistry
	if s, ok := r.callbacks[surfaceID]; ok {
		if events, ok := s[componentID]; ok {
			for _, cbID := range events {
				globalRegistry.Unregister(uint64(cbID))
			}
			delete(s, componentID)
		}
	}

	delete(r.types, handle)
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

func (r *DarwinRenderer) SetTheme(surfaceID string, theme string) {
	cSID := C.CString(surfaceID)
	defer C.free(unsafe.Pointer(cSID))
	cTheme := C.CString(theme)
	defer C.free(unsafe.Pointer(cTheme))
	C.JVSetWindowTheme(cSID, cTheme)
}

func (r *DarwinRenderer) SetRootView(surfaceID string, handle renderer.ViewHandle) {
	cSID := C.CString(surfaceID)
	defer C.free(unsafe.Pointer(cSID))

	r.mu.Lock()
	padding := r.surfacePadding[surfaceID]
	r.mu.Unlock()

	C.JVSetWindowRootView(cSID, unsafe.Pointer(handle), C.int(padding))
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

// LoadAssets registers fonts and preloads images via the native asset system.
func (r *DarwinRenderer) LoadAssets(assets []renderer.AssetSpec) {
	loadAssets(assets)
}

// UpdateMenu sets the menu bar for a surface's window.
func (r *DarwinRenderer) UpdateMenu(surfaceID string, items []renderer.MenuItemSpec) {
	updateMenu(surfaceID, items)
}

// PerformAction sends an AppKit selector through the responder chain.
func (r *DarwinRenderer) PerformAction(selector string) {
	performAction(selector)
}

// removeView removes an NSView from its superview.
func removeView(handle renderer.ViewHandle) {
	C.JVRemoveView(unsafe.Pointer(handle))
}

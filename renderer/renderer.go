package renderer

import "jview/protocol"

// Renderer is the platform-specific rendering interface.
// All methods are called on the main thread via the Dispatcher.
type Renderer interface {
	// CreateWindow opens a new native window for a surface.
	CreateWindow(spec WindowSpec)

	// DestroyWindow closes and deallocates a surface's window.
	DestroyWindow(surfaceID string)

	// CreateView creates a native view for the given component.
	// Returns a ViewHandle for later updates.
	CreateView(surfaceID string, node *RenderNode) ViewHandle

	// UpdateView updates an existing native view's properties.
	UpdateView(surfaceID string, handle ViewHandle, node *RenderNode)

	// SetChildren sets the child views of a container view.
	// childHandles are in display order.
	SetChildren(surfaceID string, parentHandle ViewHandle, childHandles []ViewHandle)

	// RemoveView removes a native view from its parent and deallocates it.
	RemoveView(surfaceID string, componentID string, handle ViewHandle)

	// GetHandle returns the current handle for a component ID, or 0 if not found.
	GetHandle(surfaceID string, componentID string) ViewHandle

	// RegisterCallback registers a Go callback for a component's event.
	// Returns a CallbackID the platform layer uses to route native events.
	RegisterCallback(surfaceID string, componentID string, eventType string, fn func(string)) CallbackID

	// UnregisterCallback removes a previously registered callback.
	UnregisterCallback(id CallbackID)

	// SetRootView sets the root view for a surface's window.
	SetRootView(surfaceID string, handle ViewHandle)

	// GetComponentType returns the component type for a handle.
	GetComponentType(handle ViewHandle) protocol.ComponentType

	// InvokeCallback programmatically triggers a registered callback for testing.
	InvokeCallback(surfaceID, componentID, eventType, data string)

	// QueryLayout returns computed frame (x, y, width, height) for a view.
	QueryLayout(surfaceID string, componentID string) LayoutInfo

	// QueryStyle returns computed style (font, color, etc.) for a view.
	QueryStyle(surfaceID string, componentID string) StyleInfo

	// LoadAssets registers fonts, preloads images, and caches asset metadata.
	LoadAssets(assets []AssetSpec)

	// SetTheme changes the visual theme for a surface's window.
	// theme: "light", "dark", or "system"
	SetTheme(surfaceID string, theme string)

	// CaptureWindow captures the window content as a PNG image.
	CaptureWindow(surfaceID string) ([]byte, error)

	// UpdateMenu sets the menu bar for a surface's window.
	UpdateMenu(surfaceID string, items []MenuItemSpec)

	// PerformAction sends an AppKit selector through the responder chain.
	PerformAction(selector string)
}

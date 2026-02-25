package renderer

import (
	"jview/protocol"
	"sync"
)

// MockRenderer records all rendering operations for test assertions.
type MockRenderer struct {
	mu sync.Mutex

	Windows  []WindowSpec
	Created  []CreatedView
	Updated  []UpdatedView
	Children []ChildrenSet
	Removed  []ViewHandle
	RootSets []RootSet

	handles    map[string]map[string]ViewHandle
	types      map[ViewHandle]protocol.ComponentType
	nextHandle ViewHandle
	callbacks  map[string]map[string]map[string]CallbackID
	callbackFn map[CallbackID]func(string)
	nextCB     CallbackID
	layouts    map[string]map[string]LayoutInfo
	styles     map[string]map[string]StyleInfo
	assets     map[string]AssetSpec
}

type CreatedView struct {
	SurfaceID string
	Node      *RenderNode
	Handle    ViewHandle
}

type UpdatedView struct {
	SurfaceID string
	Handle    ViewHandle
	Node      *RenderNode
}

type ChildrenSet struct {
	SurfaceID    string
	ParentHandle ViewHandle
	ChildHandles []ViewHandle
}

type RootSet struct {
	SurfaceID string
	Handle    ViewHandle
}

func NewMockRenderer() *MockRenderer {
	return &MockRenderer{
		handles:    make(map[string]map[string]ViewHandle),
		types:      make(map[ViewHandle]protocol.ComponentType),
		nextHandle: 1,
		callbacks:  make(map[string]map[string]map[string]CallbackID),
		callbackFn: make(map[CallbackID]func(string)),
		nextCB:     1,
	}
}

func (m *MockRenderer) CreateWindow(spec WindowSpec) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Windows = append(m.Windows, spec)
}

func (m *MockRenderer) DestroyWindow(surfaceID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.handles, surfaceID)
}

func (m *MockRenderer) CreateView(surfaceID string, node *RenderNode) ViewHandle {
	m.mu.Lock()
	defer m.mu.Unlock()
	h := m.nextHandle
	m.nextHandle++
	if m.handles[surfaceID] == nil {
		m.handles[surfaceID] = make(map[string]ViewHandle)
	}
	m.handles[surfaceID][node.ComponentID] = h
	m.types[h] = node.Type
	m.Created = append(m.Created, CreatedView{SurfaceID: surfaceID, Node: node, Handle: h})
	m.populateStyle(surfaceID, node)
	return h
}

func (m *MockRenderer) UpdateView(surfaceID string, handle ViewHandle, node *RenderNode) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Updated = append(m.Updated, UpdatedView{SurfaceID: surfaceID, Handle: handle, Node: node})
	m.populateStyle(surfaceID, node)
}

// populateStyle extracts style info from a RenderNode's StyleProps into the mock style store.
// Only sets fields that have non-zero values, preserving any previously set style info.
// Must be called with m.mu held.
func (m *MockRenderer) populateStyle(surfaceID string, node *RenderNode) {
	s := node.Style
	if s.BackgroundColor == "" && s.TextColor == "" && s.FontSize == 0 &&
		s.Opacity == 0 && s.FontWeight == "" && s.FontFamily == "" {
		return
	}
	if m.styles == nil {
		m.styles = make(map[string]map[string]StyleInfo)
	}
	if m.styles[surfaceID] == nil {
		m.styles[surfaceID] = make(map[string]StyleInfo)
	}
	info := m.styles[surfaceID][node.ComponentID]
	if s.BackgroundColor != "" {
		info.BgColor = s.BackgroundColor
	}
	if s.TextColor != "" {
		info.TextColor = s.TextColor
	}
	if s.FontSize != 0 {
		info.FontSize = s.FontSize
	}
	if s.Opacity != 0 {
		info.Opacity = s.Opacity
	}
	if s.FontWeight == "bold" {
		info.Bold = true
	}
	if s.FontFamily != "" {
		info.FontName = s.FontFamily
	}
	m.styles[surfaceID][node.ComponentID] = info
}

func (m *MockRenderer) SetChildren(surfaceID string, parentHandle ViewHandle, childHandles []ViewHandle) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Children = append(m.Children, ChildrenSet{SurfaceID: surfaceID, ParentHandle: parentHandle, ChildHandles: childHandles})
}

func (m *MockRenderer) RemoveView(surfaceID string, componentID string, handle ViewHandle) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Removed = append(m.Removed, handle)

	// Clean up handle map
	if s, ok := m.handles[surfaceID]; ok {
		delete(s, componentID)
	}

	// Clean up callbacks for this component
	if s, ok := m.callbacks[surfaceID]; ok {
		if events, ok := s[componentID]; ok {
			for _, cbID := range events {
				delete(m.callbackFn, cbID)
			}
			delete(s, componentID)
		}
	}

	delete(m.types, handle)
}

func (m *MockRenderer) GetHandle(surfaceID string, componentID string) ViewHandle {
	m.mu.Lock()
	defer m.mu.Unlock()
	if s, ok := m.handles[surfaceID]; ok {
		return s[componentID]
	}
	return 0
}

func (m *MockRenderer) RegisterCallback(surfaceID string, componentID string, eventType string, fn func(string)) CallbackID {
	m.mu.Lock()
	defer m.mu.Unlock()
	id := m.nextCB
	m.nextCB++
	m.callbackFn[id] = fn
	if m.callbacks[surfaceID] == nil {
		m.callbacks[surfaceID] = make(map[string]map[string]CallbackID)
	}
	if m.callbacks[surfaceID][componentID] == nil {
		m.callbacks[surfaceID][componentID] = make(map[string]CallbackID)
	}
	m.callbacks[surfaceID][componentID][eventType] = id
	return id
}

func (m *MockRenderer) UnregisterCallback(id CallbackID) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.callbackFn, id)
}

func (m *MockRenderer) SetRootView(surfaceID string, handle ViewHandle) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.RootSets = append(m.RootSets, RootSet{SurfaceID: surfaceID, Handle: handle})
}

func (m *MockRenderer) GetComponentType(handle ViewHandle) protocol.ComponentType {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.types[handle]
}

// InvokeCallback triggers a registered callback for testing two-way binding.
func (m *MockRenderer) InvokeCallback(surfaceID, componentID, eventType, data string) {
	m.mu.Lock()
	id := m.callbacks[surfaceID][componentID][eventType]
	fn := m.callbackFn[id]
	m.mu.Unlock()
	if fn != nil {
		fn(data)
	}
}

// HasCallback checks if a callback ID is still registered.
func (m *MockRenderer) HasCallback(id CallbackID) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	_, ok := m.callbackFn[id]
	return ok
}

// GetCallbackID returns the callback ID for a component's event, or 0.
func (m *MockRenderer) GetCallbackID(surfaceID, componentID, eventType string) CallbackID {
	m.mu.Lock()
	defer m.mu.Unlock()
	if s, ok := m.callbacks[surfaceID]; ok {
		if c, ok := s[componentID]; ok {
			return c[eventType]
		}
	}
	return 0
}

// QueryLayout returns stored layout info (set via SetLayout) for mock testing.
func (m *MockRenderer) QueryLayout(surfaceID, componentID string) LayoutInfo {
	info, _ := m.GetLayout(surfaceID, componentID)
	return info
}

// QueryStyle returns stored style info for mock testing.
func (m *MockRenderer) QueryStyle(surfaceID, componentID string) StyleInfo {
	m.mu.Lock()
	defer m.mu.Unlock()
	if s, ok := m.styles[surfaceID]; ok {
		if info, ok := s[componentID]; ok {
			return info
		}
	}
	return StyleInfo{}
}

// SetStyle stores computed style info for a component (used by mock test framework).
func (m *MockRenderer) SetStyle(surfaceID, componentID string, info StyleInfo) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.styles == nil {
		m.styles = make(map[string]map[string]StyleInfo)
	}
	if m.styles[surfaceID] == nil {
		m.styles[surfaceID] = make(map[string]StyleInfo)
	}
	m.styles[surfaceID][componentID] = info
}

// SetLayout stores computed layout info for a component (used by test framework).
func (m *MockRenderer) SetLayout(surfaceID, componentID string, info LayoutInfo) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.layouts == nil {
		m.layouts = make(map[string]map[string]LayoutInfo)
	}
	if m.layouts[surfaceID] == nil {
		m.layouts[surfaceID] = make(map[string]LayoutInfo)
	}
	m.layouts[surfaceID][componentID] = info
}

// GetLayout returns layout info for a component, if set.
func (m *MockRenderer) GetLayout(surfaceID, componentID string) (LayoutInfo, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if s, ok := m.layouts[surfaceID]; ok {
		info, ok := s[componentID]
		return info, ok
	}
	return LayoutInfo{}, false
}

// LastNode returns the most recent RenderNode for a component (from Created or Updated).
func (m *MockRenderer) LastNode(surfaceID, componentID string) *RenderNode {
	m.mu.Lock()
	defer m.mu.Unlock()
	// Check Updated in reverse order first (most recent state)
	for i := len(m.Updated) - 1; i >= 0; i-- {
		u := m.Updated[i]
		if u.SurfaceID == surfaceID && u.Node != nil && u.Node.ComponentID == componentID {
			return u.Node
		}
	}
	// Fall back to Created
	for i := len(m.Created) - 1; i >= 0; i-- {
		c := m.Created[i]
		if c.SurfaceID == surfaceID && c.Node != nil && c.Node.ComponentID == componentID {
			return c.Node
		}
	}
	return nil
}

// LoadAssets records loaded assets for test assertions.
func (m *MockRenderer) LoadAssets(assets []AssetSpec) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, a := range assets {
		if m.assets == nil {
			m.assets = make(map[string]AssetSpec)
		}
		m.assets[a.Alias] = a
	}
}

// GetAsset returns a loaded asset by alias, if any.
func (m *MockRenderer) GetAsset(alias string) (AssetSpec, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	a, ok := m.assets[alias]
	return a, ok
}

// SetTheme is a no-op for the mock renderer.
func (m *MockRenderer) SetTheme(surfaceID string, theme string) {}

// CaptureWindow is a no-op for the mock renderer.
func (m *MockRenderer) CaptureWindow(surfaceID string) ([]byte, error) {
	return nil, nil
}

// UpdateMenu is a no-op for the mock renderer.
func (m *MockRenderer) UpdateMenu(surfaceID string, items []MenuItemSpec) {}

// PerformAction is a no-op for the mock renderer.
func (m *MockRenderer) PerformAction(selector string) {}

// MockDispatcher executes functions immediately (synchronous, for tests).
type MockDispatcher struct{}

func (d *MockDispatcher) RunOnMain(fn func()) {
	fn()
}

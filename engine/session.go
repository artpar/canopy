package engine

import (
	"encoding/json"
	"fmt"
	"canopy/jlog"
	"canopy/protocol"
	"canopy/renderer"
	"sync"
)

// Session manages all surfaces and routes incoming messages.
type Session struct {
	mu       sync.Mutex
	surfaces map[string]*Surface
	rend     renderer.Renderer
	dispatch renderer.Dispatcher
	ffi      *FFIRegistry
	native   renderer.NativeProvider
	assets   *AssetRegistry
	funcDefs map[string]*FuncDef
	compDefs map[string]*protocol.DefineComponent
	pm       *ProcessManager
	cm       *ChannelManager
	recorder *Recorder
	library  *Library

	// OnAction is called when any surface triggers a server-bound event.
	OnAction func(surfaceID string, event *protocol.EventDef, data map[string]interface{})
}

func NewSession(rend renderer.Renderer, dispatch renderer.Dispatcher) *Session {
	return &Session{
		surfaces: make(map[string]*Surface),
		rend:     rend,
		dispatch: dispatch,
		funcDefs: make(map[string]*FuncDef),
		compDefs: make(map[string]*protocol.DefineComponent),
	}
}

// SetRecorder sets the recorder for this session. When set, all recordable
// messages passing through HandleMessage are written to the recorder.
func (s *Session) SetRecorder(rec *Recorder) {
	s.recorder = rec
}

// SetLibrary loads a component library into this session. All library
// component definitions are merged into compDefs and propagated to surfaces.
func (s *Session) SetLibrary(lib *Library) {
	s.library = lib
	for name, dc := range lib.Defs() {
		s.compDefs[name] = dc
	}
	for _, surf := range s.surfaces {
		surf.SetCompDefs(s.compDefs)
	}
}

// SetFFI sets the FFI registry for all surfaces created by this session.
func (s *Session) SetFFI(ffi *FFIRegistry) {
	s.ffi = ffi
}

// SetNativeProvider sets the native capabilities provider for all surfaces.
func (s *Session) SetNativeProvider(np renderer.NativeProvider) {
	s.native = np
}

// NativeProvider returns the native capabilities provider, or nil.
func (s *Session) NativeProvider() renderer.NativeProvider {
	return s.native
}

// SetProcessManager attaches a process manager to this session.
func (s *Session) SetProcessManager(pm *ProcessManager) {
	s.pm = pm
}

// SetChannelManager attaches a channel manager to this session.
func (s *Session) SetChannelManager(cm *ChannelManager) {
	s.cm = cm
}

// ChannelManager returns the attached channel manager, or nil.
func (s *Session) ChannelManager() *ChannelManager {
	return s.cm
}

// FlushPendingComponents flushes buffered updateComponents on all surfaces.
// Call this after the message stream ends to ensure all components are rendered.
func (s *Session) FlushPendingComponents() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.flushPendingComponentsLocked()
}

// flushPendingComponentsLocked is the lock-free inner implementation.
// Caller must hold s.mu.
func (s *Session) flushPendingComponentsLocked() {
	for _, surf := range s.surfaces {
		surf.FlushPendingComponents()
	}
}

// HandleMessage routes a parsed A2UI message to the appropriate surface.
// Serialized via s.mu to prevent concurrent map access from transport + MCP goroutines.
func (s *Session) HandleMessage(msg *protocol.Message) {
	s.mu.Lock()
	defer s.mu.Unlock()
	defer logRecover("session", "", "HandleMessage")
	s.handleMessageLocked(msg)
}

// handleMessageLocked does the actual work. Caller must hold s.mu.
// Methods called from here that need to call back into session (channel/process managers)
// use ForEachSurfaceLocked which accesses s.surfaces directly without re-acquiring the lock.
func (s *Session) handleMessageLocked(msg *protocol.Message) {
	jlog.Infof("session", msg.SurfaceID, "HandleMessage: type=%s", msg.Type)

	// Record to cache file if a recorder is active
	s.recorder.Record(msg)

	// Flush buffered components before any non-updateComponents message.
	// This ensures batched updateComponents calls render as a single pass.
	if msg.Type != protocol.MsgUpdateComponents {
		s.flushPendingComponentsLocked()
	}

	switch msg.Type {
	case protocol.MsgCreateSurface:
		cs := msg.Body.(protocol.CreateSurface)
		s.createSurface(cs)

	case protocol.MsgDeleteSurface:
		ds := msg.Body.(protocol.DeleteSurface)
		s.deleteSurface(ds.SurfaceID)

	case protocol.MsgUpdateComponents:
		uc := msg.Body.(protocol.UpdateComponents)
		surf, ok := s.surfaces[uc.SurfaceID]
		if !ok {
			logWarn("session", uc.SurfaceID, "unknown surface for updateComponents")
			return
		}
		surf.HandleUpdateComponents(uc)

	case protocol.MsgUpdateDataModel:
		udm := msg.Body.(protocol.UpdateDataModel)
		surf, ok := s.surfaces[udm.SurfaceID]
		if !ok {
			logWarn("session", udm.SurfaceID, "unknown surface for updateDataModel")
			return
		}
		surf.HandleUpdateDataModel(udm)

	case protocol.MsgLoadAssets:
		la := msg.Body.(protocol.LoadAssets)
		s.handleLoadAssets(la)

	case protocol.MsgLoadLibrary:
		ll := msg.Body.(protocol.LoadLibrary)
		s.handleLoadLibrary(ll)

	case protocol.MsgDefineFunction:
		df := msg.Body.(protocol.DefineFunction)
		s.handleDefineFunction(df)

	case protocol.MsgDefineComponent:
		dc := msg.Body.(protocol.DefineComponent)
		s.handleDefineComponent(dc)

	case protocol.MsgInclude:
		// Include is handled by the transport layer, not the session.
		return

	case protocol.MsgSetTheme:
		st := msg.Body.(protocol.SetTheme)
		s.dispatch.RunOnMain(func() {
			s.rend.SetTheme(st.SurfaceID, st.Theme)
		})

	case protocol.MsgTest:
		// Test messages are handled by the test runner, not the session.
		return

	case protocol.MsgCreateProcess:
		cp := msg.Body.(protocol.CreateProcess)
		if s.pm == nil {
			logWarn("session", "", "createProcess received but no ProcessManager configured")
			return
		}
		if err := s.pm.Create(cp); err != nil {
			logError("session", "", fmt.Sprintf("createProcess error: %v", err))
		}

	case protocol.MsgStopProcess:
		sp := msg.Body.(protocol.StopProcess)
		if s.pm == nil {
			logWarn("session", "", "stopProcess received but no ProcessManager configured")
			return
		}
		if err := s.pm.Stop(sp.ProcessID); err != nil {
			logError("session", "", fmt.Sprintf("stopProcess error: %v", err))
		}

	case protocol.MsgSendToProcess:
		stp := msg.Body.(protocol.SendToProcess)
		if s.pm == nil {
			logWarn("session", "", "sendToProcess received but no ProcessManager configured")
			return
		}
		innerMsg, err := protocol.ParseLine(stp.Message)
		if err != nil {
			logError("session", "", fmt.Sprintf("sendToProcess parse error: %v", err))
			return
		}
		if err := s.pm.SendTo(stp.ProcessID, innerMsg); err != nil {
			logError("session", "", fmt.Sprintf("sendToProcess error: %v", err))
		}

	case protocol.MsgCreateChannel:
		cc := msg.Body.(protocol.CreateChannel)
		if s.cm == nil {
			logWarn("session", "", "createChannel received but no ChannelManager configured")
			return
		}
		if err := s.cm.Create(cc); err != nil {
			logError("session", "", fmt.Sprintf("createChannel error: %v", err))
		}

	case protocol.MsgDeleteChannel:
		dc := msg.Body.(protocol.DeleteChannel)
		if s.cm == nil {
			logWarn("session", "", "deleteChannel received but no ChannelManager configured")
			return
		}
		if err := s.cm.Delete(dc.ChannelID); err != nil {
			logError("session", "", fmt.Sprintf("deleteChannel error: %v", err))
		}

	case protocol.MsgPublish:
		pub := msg.Body.(protocol.Publish)
		if s.cm == nil {
			logWarn("session", "", "publish received but no ChannelManager configured")
			return
		}
		if err := s.cm.Publish(pub); err != nil {
			logError("session", "", fmt.Sprintf("publish error: %v", err))
		}

	case protocol.MsgSubscribe:
		sub := msg.Body.(protocol.Subscribe)
		if s.cm == nil {
			logWarn("session", "", "subscribe received but no ChannelManager configured")
			return
		}
		if err := s.cm.Subscribe(sub); err != nil {
			logError("session", "", fmt.Sprintf("subscribe error: %v", err))
		}

	case protocol.MsgUnsubscribe:
		unsub := msg.Body.(protocol.Unsubscribe)
		if s.cm == nil {
			logWarn("session", "", "unsubscribe received but no ChannelManager configured")
			return
		}
		if err := s.cm.Unsubscribe(unsub); err != nil {
			logError("session", "", fmt.Sprintf("unsubscribe error: %v", err))
		}

	case protocol.MsgUpdateMenu:
		um := msg.Body.(protocol.UpdateMenu)
		surf, ok := s.surfaces[um.SurfaceID]
		if !ok {
			logWarn("session", um.SurfaceID, "unknown surface for updateMenu")
			return
		}
		surf.HandleUpdateMenu(um)

	case protocol.MsgUpdateToolbar:
		ut := msg.Body.(protocol.UpdateToolbar)
		surf, ok := s.surfaces[ut.SurfaceID]
		if !ok {
			logWarn("session", ut.SurfaceID, "unknown surface for updateToolbar")
			return
		}
		surf.HandleUpdateToolbar(ut)

	case protocol.MsgUpdateWindow:
		uw := msg.Body.(protocol.UpdateWindow)
		s.dispatch.RunOnMain(func() {
			s.rend.UpdateWindow(uw.SurfaceID, uw.Title, uw.MinWidth, uw.MinHeight)
		})

	case protocol.MsgSetAppMode:
		sam := msg.Body.(protocol.SetAppMode)
		var menuItems []renderer.MenuItemSpec
		if len(sam.MenuItems) > 0 {
			menuItems = s.buildStatusMenuSpecs(sam.MenuItems)
		}
		s.dispatch.RunOnMain(func() {
			s.rend.SetAppMode(sam.Mode, sam.Icon, sam.Title, 0, menuItems)
		})

	default:
		logWarn("session", "", fmt.Sprintf("unknown message type %s", msg.Type))
	}
}

// buildStatusMenuSpecs converts protocol MenuItems to renderer MenuItemSpecs,
// registering callbacks for action items at the session level.
func (s *Session) buildStatusMenuSpecs(items []protocol.MenuItem) []renderer.MenuItemSpec {
	specs := make([]renderer.MenuItemSpec, len(items))
	for i, item := range items {
		spec := renderer.MenuItemSpec{
			ID:             item.ID,
			Label:          item.Label,
			KeyEquivalent:  item.KeyEquivalent,
			KeyModifiers:   item.KeyModifiers,
			Separator:      item.Separator,
			StandardAction: item.StandardAction,
			Icon:           item.Icon,
		}
		if item.Action != nil && item.Action.Action != nil {
			action := item.Action.Action
			cbID := s.rend.RegisterCallback("__status__", "__statusmenu_"+item.ID, "click", func(data string) {
				if action.Event != nil && s.OnAction != nil {
					s.OnAction("__status__", action.Event, nil)
				}
			})
			spec.CallbackID = cbID
		}
		if len(item.Children) > 0 {
			spec.Children = s.buildStatusMenuSpecs(item.Children)
		}
		specs[i] = spec
	}
	return specs
}

func (s *Session) createSurface(cs protocol.CreateSurface) {
	if _, exists := s.surfaces[cs.SurfaceID]; exists {
		logWarn("session", cs.SurfaceID, "surface already exists")
		return
	}

	surf := NewSurface(cs.SurfaceID, s.rend, s.dispatch, s.ffi, s.assets)
	surf.SetNativeProvider(s.native)
	surf.ActionHandler = s.OnAction
	surf.SetFuncDefs(s.funcDefs)
	surf.SetCompDefs(s.compDefs)
	s.surfaces[cs.SurfaceID] = surf

	width := cs.Width
	if width == 0 {
		width = 800
	}
	height := cs.Height
	if height == 0 {
		height = 600
	}

	spec := renderer.WindowSpec{
		SurfaceID:       cs.SurfaceID,
		Title:           cs.Title,
		Width:           width,
		Height:          height,
		BackgroundColor: cs.BackgroundColor,
		Padding:         cs.Padding,
	}

	theme := cs.Theme
	if theme == "" {
		theme = "light"
	}

	s.dispatch.RunOnMain(func() {
		s.rend.CreateWindow(spec)
		s.rend.SetTheme(cs.SurfaceID, theme)
	})
}

func (s *Session) deleteSurface(surfaceID string) {
	surf, ok := s.surfaces[surfaceID]
	if !ok {
		return
	}
	surf.CleanupAll(true) // skip individual RemoveView — DestroyWindow handles it atomically
	delete(s.surfaces, surfaceID)
	s.dispatch.RunOnMain(func() {
		s.rend.DestroyWindow(surfaceID)
	})
}

func (s *Session) handleLoadLibrary(ll protocol.LoadLibrary) {
	// Lazy-init FFI registry
	if s.ffi == nil {
		s.ffi = NewFFIRegistry()
	}

	// Convert protocol types to engine types
	funcs := make([]FuncConfig, len(ll.Functions))
	for i, f := range ll.Functions {
		funcs[i] = FuncConfig{
			Name:       f.Name,
			Symbol:     f.Symbol,
			ReturnType: f.ReturnType,
			ParamTypes: f.ParamTypes,
			FixedArgs:  f.FixedArgs,
		}
	}

	if err := s.ffi.LoadLibrary(ll.Path, ll.Prefix, funcs); err != nil {
		logError("session", "", fmt.Sprintf("loadLibrary error: %v", err))
		return
	}

	// Propagate FFI registry to all existing surfaces
	for _, surf := range s.surfaces {
		surf.SetFFI(s.ffi)
	}
}

func (s *Session) handleDefineFunction(df protocol.DefineFunction) {
	if _, exists := s.funcDefs[df.Name]; exists {
		logWarn("session", "", fmt.Sprintf("redefining function %s", df.Name))
	}
	body := df.Body
	// Defensive: if the body was double-encoded as a JSON string, auto-parse it
	if str, ok := body.(string); ok {
		var parsed interface{}
		if err := json.Unmarshal([]byte(str), &parsed); err == nil {
			logWarn("session", "", fmt.Sprintf("defineFunction %s: body was a JSON string, auto-parsed to object", df.Name))
			body = parsed
		} else {
			logWarn("session", "", fmt.Sprintf("defineFunction %s: body is a malformed JSON string (not parseable): %v", df.Name, err))
		}
	}
	def := &FuncDef{
		Name:   df.Name,
		Params: df.Params,
		Body:   body,
	}
	s.funcDefs[df.Name] = def
	// Propagate to all existing surfaces
	for _, surf := range s.surfaces {
		surf.SetFuncDefs(s.funcDefs)
	}
}

func (s *Session) handleDefineComponent(dc protocol.DefineComponent) {
	if _, exists := s.compDefs[dc.Name]; exists {
		logWarn("session", "", fmt.Sprintf("redefining component %s", dc.Name))
	}
	s.compDefs[dc.Name] = &dc
	// Persist to library during generation (when recorder is active)
	if s.recorder != nil && s.library != nil {
		if err := s.library.Save(&dc); err != nil {
			logError("session", "", fmt.Sprintf("library save error: %v", err))
		}
	}
	// Propagate to all existing surfaces
	for _, surf := range s.surfaces {
		surf.SetCompDefs(s.compDefs)
	}
}

// GetSurface returns the surface with the given ID, or nil if not found.
// Safe to call from any goroutine.
func (s *Session) GetSurface(id string) *Surface {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.surfaces[id]
}

// getSurfaceLocked returns the surface without acquiring the mutex.
// Caller must hold s.mu.
func (s *Session) getSurfaceLocked(id string) *Surface {
	return s.surfaces[id]
}

// SurfaceIDs returns the IDs of all active surfaces.
// Safe to call from any goroutine.
func (s *Session) SurfaceIDs() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.surfaceIDsLocked()
}

// surfaceIDsLocked returns surface IDs without acquiring the mutex.
// Caller must hold s.mu or be called from within HandleMessage.
func (s *Session) surfaceIDsLocked() []string {
	ids := make([]string, 0, len(s.surfaces))
	for id := range s.surfaces {
		ids = append(ids, id)
	}
	return ids
}

// ForEachSurface calls fn for each active surface. Thread-safe.
// Used by code that may be called from goroutines outside HandleMessage.
func (s *Session) ForEachSurface(fn func(surfaceID string, surf *Surface)) {
	s.mu.Lock()
	surfs := make(map[string]*Surface, len(s.surfaces))
	for id, surf := range s.surfaces {
		surfs[id] = surf
	}
	s.mu.Unlock()
	for id, surf := range surfs {
		fn(id, surf)
	}
}

// forEachSurfaceLocked calls fn for each active surface.
// Caller must hold s.mu. Used by channel/process managers called from within HandleMessage.
func (s *Session) forEachSurfaceLocked(fn func(surfaceID string, surf *Surface)) {
	for id, surf := range s.surfaces {
		fn(id, surf)
	}
}

func (s *Session) handleLoadAssets(la protocol.LoadAssets) {
	if s.assets == nil {
		s.assets = NewAssetRegistry()
	}

	specs := make([]renderer.AssetSpec, len(la.Assets))
	for i, a := range la.Assets {
		s.assets.Register(a.Alias, a.Kind, a.Src)
		specs[i] = renderer.AssetSpec{
			Alias: a.Alias,
			Kind:  a.Kind,
			Src:   a.Src,
		}
	}

	// Propagate to all existing surfaces
	for _, surf := range s.surfaces {
		surf.SetAssets(s.assets)
	}

	s.dispatch.RunOnMain(func() {
		s.rend.LoadAssets(specs)
	})
}

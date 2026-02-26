package engine

import (
	"encoding/json"
	"fmt"
	"jview/protocol"
	"jview/renderer"
)

// Session manages all surfaces and routes incoming messages.
type Session struct {
	surfaces map[string]*Surface
	rend     renderer.Renderer
	dispatch renderer.Dispatcher
	ffi      *FFIRegistry
	assets   *AssetRegistry
	funcDefs map[string]*FuncDef
	compDefs map[string]*protocol.DefineComponent
	pm       *ProcessManager
	cm       *ChannelManager

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

// SetFFI sets the FFI registry for all surfaces created by this session.
func (s *Session) SetFFI(ffi *FFIRegistry) {
	s.ffi = ffi
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
	for _, surf := range s.surfaces {
		surf.FlushPendingComponents()
	}
}

// HandleMessage routes a parsed A2UI message to the appropriate surface.
func (s *Session) HandleMessage(msg *protocol.Message) {
	defer logRecover("session", "", "HandleMessage")

	// Flush buffered components before any non-updateComponents message.
	// This ensures batched updateComponents calls render as a single pass.
	if msg.Type != protocol.MsgUpdateComponents {
		s.FlushPendingComponents()
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

	default:
		logWarn("session", "", fmt.Sprintf("unknown message type %s", msg.Type))
	}
}

func (s *Session) createSurface(cs protocol.CreateSurface) {
	if _, exists := s.surfaces[cs.SurfaceID]; exists {
		logWarn("session", cs.SurfaceID, "surface already exists")
		return
	}

	surf := NewSurface(cs.SurfaceID, s.rend, s.dispatch, s.ffi, s.assets)
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

	s.dispatch.RunOnMain(func() {
		s.rend.CreateWindow(spec)
	})
}

func (s *Session) deleteSurface(surfaceID string) {
	surf, ok := s.surfaces[surfaceID]
	if !ok {
		return
	}
	surf.CleanupAll()
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
	// Propagate to all existing surfaces
	for _, surf := range s.surfaces {
		surf.SetCompDefs(s.compDefs)
	}
}

// GetSurface returns the surface with the given ID, or nil if not found.
func (s *Session) GetSurface(id string) *Surface {
	return s.surfaces[id]
}

// SurfaceIDs returns the IDs of all active surfaces.
func (s *Session) SurfaceIDs() []string {
	ids := make([]string, 0, len(s.surfaces))
	for id := range s.surfaces {
		ids = append(ids, id)
	}
	return ids
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

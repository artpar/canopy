package engine

import (
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

// HandleMessage routes a parsed A2UI message to the appropriate surface.
func (s *Session) HandleMessage(msg *protocol.Message) {
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
	def := &FuncDef{
		Name:   df.Name,
		Params: df.Params,
		Body:   df.Body,
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

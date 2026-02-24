package engine

import (
	"jview/protocol"
	"jview/renderer"
	"log"
)

// Session manages all surfaces and routes incoming messages.
type Session struct {
	surfaces map[string]*Surface
	rend     renderer.Renderer
	dispatch renderer.Dispatcher
	ffi      *FFIRegistry
	assets   *AssetRegistry

	// OnAction is called when any surface triggers a server-bound event.
	OnAction func(surfaceID string, event *protocol.EventDef, data map[string]interface{})
}

func NewSession(rend renderer.Renderer, dispatch renderer.Dispatcher) *Session {
	return &Session{
		surfaces: make(map[string]*Surface),
		rend:     rend,
		dispatch: dispatch,
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
			log.Printf("session: unknown surface %s for updateComponents", uc.SurfaceID)
			return
		}
		surf.HandleUpdateComponents(uc)

	case protocol.MsgUpdateDataModel:
		udm := msg.Body.(protocol.UpdateDataModel)
		surf, ok := s.surfaces[udm.SurfaceID]
		if !ok {
			log.Printf("session: unknown surface %s for updateDataModel", udm.SurfaceID)
			return
		}
		surf.HandleUpdateDataModel(udm)

	case protocol.MsgLoadAssets:
		la := msg.Body.(protocol.LoadAssets)
		s.handleLoadAssets(la)

	case protocol.MsgLoadLibrary:
		ll := msg.Body.(protocol.LoadLibrary)
		s.handleLoadLibrary(ll)

	case protocol.MsgSetTheme:
		// Phase 3: theme support
		log.Printf("session: setTheme not yet implemented")

	case protocol.MsgTest:
		// Test messages are handled by the test runner, not the session.
		return

	default:
		log.Printf("session: unknown message type %s", msg.Type)
	}
}

func (s *Session) createSurface(cs protocol.CreateSurface) {
	if _, exists := s.surfaces[cs.SurfaceID]; exists {
		log.Printf("session: surface %s already exists", cs.SurfaceID)
		return
	}

	surf := NewSurface(cs.SurfaceID, s.rend, s.dispatch, s.ffi, s.assets)
	surf.ActionHandler = s.OnAction
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
	if _, ok := s.surfaces[surfaceID]; !ok {
		return
	}
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
		log.Printf("session: loadLibrary error: %v", err)
		return
	}

	// Propagate FFI registry to all existing surfaces
	for _, surf := range s.surfaces {
		surf.SetFFI(s.ffi)
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

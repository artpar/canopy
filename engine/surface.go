package engine

import (
	"encoding/json"
	"fmt"
	"jview/jlog"
	"jview/protocol"
	"jview/renderer"
)

// Surface manages a single A2UI surface: its component tree, data model, and bindings.
type Surface struct {
	id        string
	tree      *Tree
	dm        *DataModel
	tracker   *BindingTracker
	resolver  *Resolver
	validator *Validator
	rend      renderer.Renderer
	dispatch  renderer.Dispatcher
	ffi       *FFIRegistry
	native    renderer.NativeProvider
	assets    *AssetRegistry

	// activeCallbacks tracks registered callbacks: componentID → eventType → CallbackID
	activeCallbacks map[string]map[string]renderer.CallbackID

	// lastToolbarMsg stores the most recent toolbar message for re-resolution on data changes
	lastToolbarMsg *protocol.UpdateToolbar

	// validationErrors tracks current validation errors: componentID → []errorMessages
	validationErrors map[string][]string

	// funcDefs holds user-defined functions for the evaluator
	funcDefs map[string]*FuncDef

	// compDefs holds user-defined component templates
	compDefs map[string]*protocol.DefineComponent

	// forEachMetas stores original component + template tree for forEach re-expansion
	forEachMetas map[string]*forEachMeta

	// reexpanding prevents recursive re-expansion in renderComponents
	reexpanding bool

	// pendingComponents buffers expanded components from consecutive updateComponents calls.
	// Flushed as a single render pass when a different message type arrives.
	pendingComponents []protocol.Component

	// ActionHandler is called when a component triggers a server-bound event.
	ActionHandler func(surfaceID string, event *protocol.EventDef, data map[string]interface{})
}

// forEachMeta stores the original component and template tree for forEach re-expansion.
type forEachMeta struct {
	component    protocol.Component
	templateTree []protocol.Component
}

func NewSurface(id string, rend renderer.Renderer, dispatch renderer.Dispatcher, ffi *FFIRegistry, assets *AssetRegistry) *Surface {
	dm := NewDataModel()
	tracker := NewBindingTracker()
	evaluator := NewEvaluator(dm)
	evaluator.FFI = ffi
	resolver := NewResolver(dm, tracker, evaluator)
	resolver.assets = assets
	return &Surface{

		id:               id,
		tree:             NewTree(),
		dm:               dm,
		tracker:          tracker,
		resolver:         resolver,
		validator:        NewValidator(),
		rend:             rend,
		dispatch:         dispatch,
		ffi:              ffi,
		assets:           assets,
		activeCallbacks:  make(map[string]map[string]renderer.CallbackID),
		validationErrors: make(map[string][]string),
		funcDefs:         make(map[string]*FuncDef),
		compDefs:         make(map[string]*protocol.DefineComponent),
		forEachMetas:     make(map[string]*forEachMeta),
	}
}

// ID returns the surface ID.
func (s *Surface) ID() string {
	return s.id
}

// Tree returns the component tree.
func (s *Surface) Tree() *Tree {
	return s.tree
}

// DM returns the data model.
func (s *Surface) DM() *DataModel {
	return s.dm
}

// Resolver returns the resolver.
func (s *Surface) Resolver() *Resolver {
	return s.resolver
}

// SetFFI updates the FFI registry for this surface and its evaluator.
func (s *Surface) SetFFI(ffi *FFIRegistry) {
	s.ffi = ffi
	s.resolver.evaluator.FFI = ffi
}

// SetNativeProvider updates the native capabilities provider for this surface.
func (s *Surface) SetNativeProvider(np renderer.NativeProvider) {
	s.native = np
	s.resolver.evaluator.Native = np
}

// SetAssets updates the asset registry for this surface and its resolver.
func (s *Surface) SetAssets(assets *AssetRegistry) {
	s.assets = assets
	s.resolver.assets = assets
}

// SetFuncDefs updates the user-defined functions for this surface's evaluator.
func (s *Surface) SetFuncDefs(defs map[string]*FuncDef) {
	s.funcDefs = defs
	s.resolver.evaluator.customFuncs = defs
}

// SetCompDefs updates the user-defined component templates for this surface.
func (s *Surface) SetCompDefs(defs map[string]*protocol.DefineComponent) {
	s.compDefs = defs
}

// HandleUpdateComponents buffers component definitions for deferred rendering.
// Components are expanded (instances + templates) and accumulated. The actual
// render pass happens when FlushPendingComponents is called — either by the
// session before processing a non-updateComponents message, or explicitly.
func (s *Surface) HandleUpdateComponents(msg protocol.UpdateComponents) {
	jlog.Infof("surface", s.id, "HandleUpdateComponents: %d raw components", len(msg.Components))
	for i, c := range msg.Components {
		jlog.Debugf("surface", s.id, "  comp[%d]: id=%s type=%s children=%v", i, c.ComponentID, c.Type, c.Children)
	}
	comps := s.expandComponentInstances(msg.Components)
	expanded := s.expandTemplates(comps)
	jlog.Infof("surface", s.id, "HandleUpdateComponents: %d expanded → %d pending total", len(expanded), len(s.pendingComponents)+len(expanded))
	s.pendingComponents = append(s.pendingComponents, expanded...)
}

// FlushPendingComponents renders all buffered components as a single pass.
// This prevents intermediate root-wrapping when components arrive in batches.
func (s *Surface) FlushPendingComponents() {
	if len(s.pendingComponents) == 0 {
		return
	}
	pending := s.pendingComponents
	s.pendingComponents = nil

	oldRoots := s.tree.RootIDs()
	prevRoots := make([]string, len(oldRoots))
	copy(prevRoots, oldRoots)
	changed := s.tree.Update(pending)
	removed := s.tree.Prune(prevRoots, changed)
	jlog.Infof("surface", s.id, "FlushPendingComponents: %d changed, %d removed", len(changed), len(removed))
	if len(removed) > 0 {
		s.cleanupComponents(removed)
	}
	s.renderComponents(changed)
}

// cleanupComponents removes orphaned components: unregisters callbacks, bindings,
// validation errors, and dispatches RemoveView for each.
func (s *Surface) cleanupComponents(removedIDs []string) {
	// Unregister callbacks and bindings (Go-side, no dispatch needed)
	for _, id := range removedIDs {
		if events, exists := s.activeCallbacks[id]; exists {
			for _, cbID := range events {
				s.rend.UnregisterCallback(cbID)
			}
			delete(s.activeCallbacks, id)
		}
		s.tracker.Unregister(id)
		delete(s.validationErrors, id)
	}

	// Dispatch RemoveView on main thread for components that have handles
	s.dispatch.RunOnMain(func() {
		for _, id := range removedIDs {
			handle := s.rend.GetHandle(s.id, id)
			if handle != 0 {
				s.rend.RemoveView(s.id, id, handle)
			}
		}
	})
}

// CleanupAll unregisters all callbacks and bindings for every component in the tree,
// and dispatches RemoveView for each component with a handle.
// Called before surface deletion to prevent stale callback invocations and resource leaks.
func (s *Surface) CleanupAll() {
	allIDs := s.tree.All()
	for _, id := range allIDs {
		if events, exists := s.activeCallbacks[id]; exists {
			for _, cbID := range events {
				s.rend.UnregisterCallback(cbID)
			}
			delete(s.activeCallbacks, id)
		}
		s.tracker.Unregister(id)
		delete(s.validationErrors, id)
	}

	// Dispatch RemoveView on main thread for components that have handles
	s.dispatch.RunOnMain(func() {
		for _, id := range allIDs {
			handle := s.rend.GetHandle(s.id, id)
			if handle != 0 {
				s.rend.RemoveView(s.id, id, handle)
			}
		}
	})
}

// HandleUpdateDataModel applies data model operations and re-renders affected components.
func (s *Surface) HandleUpdateDataModel(msg protocol.UpdateDataModel) {
	evaluator := NewEvaluator(s.dm)
	evaluator.FFI = s.ffi
	evaluator.Native = s.native
	evaluator.customFuncs = s.funcDefs
	var allChanged []string
	for _, op := range msg.Ops {
		var changed []string
		var err error
		switch op.Op {
		case "add", "replace":
			value, resolveErr := evaluator.resolveArg(op.Value)
			if resolveErr != nil {
				logWarn("datamodel", s.id, fmt.Sprintf("resolve value error: %v", resolveErr))
				continue
			}
			changed, err = s.dm.Set(op.Path, value)
		case "remove":
			changed, err = s.dm.Delete(op.Path)
		default:
			logWarn("datamodel", s.id, fmt.Sprintf("unknown data op %q", op.Op))
			continue
		}
		if err != nil {
			logWarn("datamodel", s.id, fmt.Sprintf("data op error: %v", err))
			continue
		}
		allChanged = append(allChanged, changed...)
	}

	if len(allChanged) == 0 {
		return
	}

	affected := s.tracker.Affected(allChanged)
	if len(affected) > 0 {
		s.renderComponents(affected)
	}
}

// HandleUpdateToolbar registers callbacks for toolbar items and dispatches
// the toolbar update to the renderer.
func (s *Surface) HandleUpdateToolbar(msg protocol.UpdateToolbar) {
	// Store for re-dispatch when bound data changes
	s.lastToolbarMsg = &msg

	// Unregister old toolbar callbacks and bindings
	if old, exists := s.activeCallbacks["__toolbar__"]; exists {
		for _, cbID := range old {
			s.rend.UnregisterCallback(cbID)
		}
		delete(s.activeCallbacks, "__toolbar__")
	}
	s.tracker.Unregister("__toolbar__")

	specs := make([]renderer.ToolbarItemSpec, len(msg.Items))
	for i, item := range msg.Items {
		spec := renderer.ToolbarItemSpec{
			ID:             item.ID,
			Icon:           item.Icon,
			Label:          item.Label,
			StandardAction: item.StandardAction,
			Separator:      item.Separator,
			Flexible:       item.Flexible,
			SearchField:    item.SearchField,
			Enabled:        s.resolver.resolveBoolDefault("__toolbar__", item.Enabled, true),
			Selected:       s.resolver.resolveBool("__toolbar__", item.Selected),
			HasToggle:      item.Selected != nil,
			Bordered:       item.Bordered,
		}
		if item.Action != nil && item.Action.Action != nil {
			action := item.Action.Action
			itemID := item.ID
			cbID := s.rend.RegisterCallback(s.id, "__toolbar_"+item.ID, "click", func(data string) {
				jlog.Infof("toolbar", s.id, "toolbar click: %s", itemID)
				if action.StandardAction != "" {
					s.dispatch.RunOnMain(func() {
						s.rend.PerformAction(action.StandardAction)
					})
				} else if action.Event != nil {
					resolved := s.resolveDataRefs(action.Event)
					if s.ActionHandler != nil {
						s.ActionHandler(s.id, action.Event, resolved)
					}
				} else if action.FunctionCall != nil {
					s.executeFunctionCall(action.FunctionCall)
				}
			})
			spec.CallbackID = cbID
			s.trackCallback("__toolbar__", item.ID, cbID)
		}
		if item.SearchField && item.DataBinding != "" {
			binding := item.DataBinding
			cbID := s.rend.RegisterCallback(s.id, "__toolbar_search_"+item.ID, "change", func(value string) {
				jlog.Infof("search", s.id, "search callback: binding=%s value=%q", binding, value)
				changed, err := s.dm.Set(binding, value)
				if err != nil {
					logWarn("binding", s.id, fmt.Sprintf("toolbar search binding error: %v", err))
					return
				}
				jlog.Infof("search", s.id, "search changed paths: %v", changed)
				affected := s.tracker.Affected(changed)
				jlog.Infof("search", s.id, "search affected components: %v", affected)
				if len(affected) > 0 {
					s.renderComponents(affected)
				}
			})
			spec.SearchCallbackID = cbID
			s.trackCallback("__toolbar__", "search_"+item.ID, cbID)
		}
		specs[i] = spec
	}

	s.dispatch.RunOnMain(func() {
		s.rend.UpdateToolbar(s.id, specs)
	})
}

// HandleUpdateMenu registers callbacks for menu items with actions and dispatches
// the menu update to the renderer.
func (s *Surface) HandleUpdateMenu(msg protocol.UpdateMenu) {
	// Unregister old menu callbacks
	if old, exists := s.activeCallbacks["__menu__"]; exists {
		for _, cbID := range old {
			s.rend.UnregisterCallback(cbID)
		}
		delete(s.activeCallbacks, "__menu__")
	}

	items := s.buildMenuSpecs(msg.Items)

	s.dispatch.RunOnMain(func() {
		s.rend.UpdateMenu(s.id, items)
	})
}

// buildMenuSpecs recursively walks MenuItem tree, registering callbacks for action items.
func (s *Surface) buildMenuSpecs(items []protocol.MenuItem) []renderer.MenuItemSpec {
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
			Disabled:       s.resolver.resolveBool("__menu__", item.Disabled),
		}
		if item.Action != nil && item.Action.Action != nil {
			action := item.Action.Action
			cbID := s.rend.RegisterCallback(s.id, "__menu_"+item.ID, "click", func(data string) {
				if action.Event != nil {
					resolved := s.resolveDataRefs(action.Event)
					if s.ActionHandler != nil {
						s.ActionHandler(s.id, action.Event, resolved)
					}
				} else if action.FunctionCall != nil {
					s.executeFunctionCall(action.FunctionCall)
				}
			})
			spec.CallbackID = cbID
			s.trackCallback("__menu__", item.ID, cbID)
		}
		if len(item.Children) > 0 {
			spec.Children = s.buildMenuSpecs(item.Children)
		}
		specs[i] = spec
	}
	return specs
}

// buildContextMenuSpecs builds menu specs for a component's context menu,
// using a namespace that avoids collision with the menu bar callbacks.
func (s *Surface) buildContextMenuSpecs(compID string, items []protocol.MenuItem) []renderer.MenuItemSpec {
	specs := make([]renderer.MenuItemSpec, len(items))
	for i, item := range items {
		spec := renderer.MenuItemSpec{
			ID:             item.ID,
			Label:          item.Label,
			Separator:      item.Separator,
			StandardAction: item.StandardAction,
			Icon:           item.Icon,
			Disabled:       s.resolver.resolveBool(compID, item.Disabled),
		}
		if item.Action != nil && item.Action.Action != nil {
			action := item.Action.Action
			cbID := s.rend.RegisterCallback(s.id, "__ctx_"+compID+"_"+item.ID, "click", func(data string) {
				if action.Event != nil {
					resolved := s.resolveDataRefs(action.Event)
					if s.ActionHandler != nil {
						s.ActionHandler(s.id, action.Event, resolved)
					}
				} else if action.FunctionCall != nil {
					s.executeFunctionCall(action.FunctionCall)
				}
			})
			spec.CallbackID = cbID
			s.trackCallback(compID, "__ctx_"+item.ID, cbID)
		}
		if len(item.Children) > 0 {
			spec.Children = s.buildContextMenuSpecs(compID, item.Children)
		}
		specs[i] = spec
	}
	return specs
}

// renderComponents resolves and dispatches render operations for the given component IDs.
func (s *Surface) renderComponents(componentIDs []string) {
	jlog.Infof("render", s.id, "renderComponents: %v", componentIDs)

	// Re-dispatch toolbar if its bindings are affected
	if s.lastToolbarMsg != nil {
		var remaining []string
		for _, id := range componentIDs {
			if id == "__toolbar__" {
				s.HandleUpdateToolbar(*s.lastToolbarMsg)
			} else {
				remaining = append(remaining, id)
			}
		}
		if len(remaining) == 0 {
			return
		}
		componentIDs = remaining
	}

	// Re-expand forEach parents whose data source changed.
	// Skip during re-expansion to prevent recursion.
	if !s.reexpanding {
		var forEachIDs []string
		var otherIDs []string
		for _, id := range componentIDs {
			if _, ok := s.forEachMetas[id]; ok {
				forEachIDs = append(forEachIDs, id)
			} else {
				otherIDs = append(otherIDs, id)
			}
		}
		if len(forEachIDs) > 0 {
			s.reexpanding = true
			for _, id := range forEachIDs {
				meta := s.forEachMetas[id]
				comps := []protocol.Component{meta.component}
				comps = append(comps, meta.templateTree...)
				s.HandleUpdateComponents(protocol.UpdateComponents{Components: comps})
			}
			// Internal re-expansion must render immediately (not batched)
			s.FlushPendingComponents()
			s.reexpanding = false
			if len(otherIDs) == 0 {
				return
			}
			componentIDs = otherIDs
		}
	}

	type renderWork struct {
		node *renderer.RenderNode
		comp *protocol.Component
	}

	// Build work items and sort: children before parents (leaves first).
	// We do a topological sort based on the children references.
	nodeMap := make(map[string]*renderWork, len(componentIDs))
	for _, id := range componentIDs {
		comp, ok := s.tree.Get(id)
		if !ok {
			continue
		}
		node := s.resolver.Resolve(comp)
		// Attach validation errors if any
		if errs, ok := s.validationErrors[id]; ok {
			node.Props.ValidationErrors = errs
		}
		s.registerCallbacks(comp, node)
		nodeMap[id] = &renderWork{node: node, comp: comp}
	}

	// Topological order: leaves first, roots last
	var ordered []*renderWork
	visited := make(map[string]bool)
	var visit func(id string)
	visit = func(id string) {
		if visited[id] {
			return
		}
		visited[id] = true
		w, ok := nodeMap[id]
		if !ok {
			return
		}
		// Visit children first
		for _, childID := range w.node.ChildIDs {
			visit(childID)
		}
		ordered = append(ordered, w)
	}
	for _, id := range componentIDs {
		visit(id)
	}

	s.dispatch.RunOnMain(func() {
		// First pass: create/update all views (leaves first)
		for _, w := range ordered {
			func() {
				defer logRecover("render", s.id, "CreateView/UpdateView "+w.node.ComponentID)
				handle := s.rend.GetHandle(s.id, w.node.ComponentID)
				if handle == 0 {
					h := s.rend.CreateView(s.id, w.node)
					if h == 0 {
						logWarn("render", s.id, fmt.Sprintf("CreateView returned 0 for %s (type %s)", w.node.ComponentID, w.node.Type))
					}
				} else {
					s.rend.UpdateView(s.id, handle, w.node)
				}
			}()
		}

		// Second pass: set children for containers (now all children exist)
		for _, w := range ordered {
			if len(w.node.ChildIDs) == 0 {
				continue
			}
			func() {
				defer logRecover("render", s.id, "SetChildren "+w.node.ComponentID)
				parentHandle := s.rend.GetHandle(s.id, w.node.ComponentID)
				if parentHandle == 0 {
					return
				}
				childHandles := make([]renderer.ViewHandle, 0, len(w.node.ChildIDs))
				for _, childID := range w.node.ChildIDs {
					ch := s.rend.GetHandle(s.id, childID)
					if ch != 0 {
						childHandles = append(childHandles, ch)
					}
				}
				if len(childHandles) > 0 {
					s.rend.SetChildren(s.id, parentHandle, childHandles)
				}
			}()
		}

		// Set root view(s)
		roots := s.tree.RootIDs()
		if len(roots) == 1 {
			h := s.rend.GetHandle(s.id, roots[0])
			if h != 0 {
				s.rend.SetRootView(s.id, h)
			}
		} else if len(roots) > 1 {
			wrapperNode := &renderer.RenderNode{
				ComponentID: "__root_wrapper__",
				Type:        protocol.CompColumn,
				Props: renderer.ResolvedProps{
					Gap:     8,
					Padding: 16,
				},
			}
			wrapperHandle := s.rend.GetHandle(s.id, "__root_wrapper__")
			if wrapperHandle == 0 {
				wrapperHandle = s.rend.CreateView(s.id, wrapperNode)
			}
			if wrapperHandle == 0 {
				logWarn("render", s.id, "CreateView returned 0 for __root_wrapper__")
				return
			}
			var rootHandles []renderer.ViewHandle
			for _, rid := range roots {
				h := s.rend.GetHandle(s.id, rid)
				if h != 0 {
					rootHandles = append(rootHandles, h)
				}
			}
			s.rend.SetChildren(s.id, wrapperHandle, rootHandles)
			s.rend.SetRootView(s.id, wrapperHandle)
		}
	})
}

// resolveDataRefs reads each DataRefs path from the data model and returns a map.
func (s *Surface) resolveDataRefs(event *protocol.EventDef) map[string]interface{} {
	result := make(map[string]interface{}, len(event.DataRefs))
	for _, path := range event.DataRefs {
		if val, ok := s.dm.Get(path); ok {
			result[path] = val
		}
	}
	return result
}

// executeFunctionCall handles client-side function calls from button actions.
func (s *Surface) executeFunctionCall(fc *protocol.ActionFuncCall) {
	defer logRecover("functioncall", s.id, fc.Call)
	jlog.Infof("functioncall", s.id, "executeFunctionCall: %s", fc.Call)

	switch fc.Call {
	case "updateDataModel":
		s.executeUpdateDataModel(fc.Args)
	case "setTheme":
		s.executeSetTheme(fc.Args)
	case "updateWindow":
		s.executeUpdateWindow(fc.Args)
	default:
		logWarn("functioncall", s.id, fmt.Sprintf("unknown functionCall: %s", fc.Call))
	}
}

// executeUpdateDataModel applies data model operations from a functionCall's args.
// Args is expected to be map[string]interface{} with an "ops" key containing an array of ops.
// Each op has {op, path, value} where value can be a dynamic (functionCall/path ref).
func (s *Surface) executeUpdateDataModel(args interface{}) {
	argsMap, ok := args.(map[string]interface{})
	if !ok {
		logWarn("functioncall", s.id, "updateDataModel args not a map")
		return
	}
	opsRaw, ok := argsMap["ops"]
	if !ok {
		logWarn("functioncall", s.id, "updateDataModel missing ops")
		return
	}
	ops, ok := opsRaw.([]interface{})
	if !ok {
		logWarn("functioncall", s.id, "updateDataModel ops not an array")
		return
	}

	evaluator := NewEvaluator(s.dm)
	evaluator.FFI = s.ffi
	evaluator.Native = s.native
	evaluator.customFuncs = s.funcDefs
	var allChanged []string

	for i, opRaw := range ops {
		opMap, ok := opRaw.(map[string]interface{})
		if !ok {
			jlog.Warnf("functioncall", s.id, "op[%d] not a map", i)
			continue
		}
		opType, _ := opMap["op"].(string)
		path, _ := opMap["path"].(string)
		if opType == "" || path == "" {
			jlog.Warnf("functioncall", s.id, "op[%d] missing op=%q path=%q", i, opType, path)
			continue
		}

		switch opType {
		case "add", "replace":
			value, err := evaluator.resolveArg(opMap["value"])
			if err != nil {
				logWarn("functioncall", s.id, fmt.Sprintf("op[%d] resolve value error for %s: %v", i, path, err))
				continue
			}
			jlog.Infof("functioncall", s.id, "op[%d] %s %s = %T", i, opType, path, value)
			changed, err := s.dm.Set(path, value)
			if err != nil {
				logWarn("datamodel", s.id, fmt.Sprintf("op[%d] data op error for %s: %v", i, path, err))
				continue
			}
			allChanged = append(allChanged, changed...)
		case "remove":
			changed, err := s.dm.Delete(path)
			if err != nil {
				logWarn("datamodel", s.id, fmt.Sprintf("data op error: %v", err))
				continue
			}
			allChanged = append(allChanged, changed...)
		}
	}

	if len(allChanged) == 0 {
		return
	}
	affected := s.tracker.Affected(allChanged)
	if len(affected) > 0 {
		s.renderComponents(affected)
	}
}

// executeUpdateWindow sets window properties via the renderer.
func (s *Surface) executeUpdateWindow(args interface{}) {
	argsMap, ok := args.(map[string]interface{})
	if !ok {
		logWarn("functioncall", s.id, "updateWindow args not a map")
		return
	}
	evaluator := NewEvaluator(s.dm)
	evaluator.FFI = s.ffi
	evaluator.Native = s.native
	evaluator.customFuncs = s.funcDefs
	title := ""
	if titleRaw, ok := argsMap["title"]; ok {
		resolved, err := evaluator.resolveArg(titleRaw)
		if err == nil {
			title = toString(resolved)
		}
	}
	minWidth := 0
	if v, ok := argsMap["minWidth"]; ok {
		resolved, err := evaluator.resolveArg(v)
		if err == nil {
			if f, err := toFloat(resolved); err == nil {
				minWidth = int(f)
			}
		}
	}
	minHeight := 0
	if v, ok := argsMap["minHeight"]; ok {
		resolved, err := evaluator.resolveArg(v)
		if err == nil {
			if f, err := toFloat(resolved); err == nil {
				minHeight = int(f)
			}
		}
	}
	s.dispatch.RunOnMain(func() {
		s.rend.UpdateWindow(s.id, title, minWidth, minHeight)
	})
}

// executeSetTheme changes the window theme via the renderer.
// Args is expected to be map[string]interface{} with a "theme" key ("light", "dark", or "system").
func (s *Surface) executeSetTheme(args interface{}) {
	argsMap, ok := args.(map[string]interface{})
	if !ok {
		logWarn("functioncall", s.id, "setTheme args not a map")
		return
	}
	theme, ok := argsMap["theme"].(string)
	if !ok || theme == "" {
		logWarn("functioncall", s.id, "setTheme missing or invalid theme")
		return
	}
	s.dispatch.RunOnMain(func() {
		s.rend.SetTheme(s.id, theme)
	})
}

func (s *Surface) registerCallbacks(comp *protocol.Component, node *renderer.RenderNode) {
	node.Callbacks = make(map[string]renderer.CallbackID)

	// Unregister any existing callbacks for this component before re-registering
	if old, exists := s.activeCallbacks[comp.ComponentID]; exists {
		for _, cbID := range old {
			s.rend.UnregisterCallback(cbID)
		}
		delete(s.activeCallbacks, comp.ComponentID)
	}

	switch comp.Type {
	case protocol.CompButton:
		if comp.Props.OnClick != nil && comp.Props.OnClick.Action != nil {
			action := comp.Props.OnClick.Action
			cbID := s.rend.RegisterCallback(s.id, comp.ComponentID, "click", func(data string) {
				if action.StandardAction != "" {
					s.dispatch.RunOnMain(func() {
						s.rend.PerformAction(action.StandardAction)
					})
				} else if action.Event != nil {
					resolved := s.resolveDataRefs(action.Event)
					if s.ActionHandler != nil {
						s.ActionHandler(s.id, action.Event, resolved)
					}
				} else if action.FunctionCall != nil {
					s.executeFunctionCall(action.FunctionCall)
				}
			})
			node.Callbacks["click"] = cbID
			s.trackCallback(comp.ComponentID, "click", cbID)
		}

	case protocol.CompTextField:
		if comp.Props.DataBinding != "" {
			binding := comp.Props.DataBinding
			compID := comp.ComponentID
			validations := comp.Props.Validations
			cbID := s.rend.RegisterCallback(s.id, comp.ComponentID, "change", func(value string) {
				changed, err := s.dm.Set(binding, value)
				if err != nil {
					logWarn("binding", s.id, fmt.Sprintf("binding set error: %v", err))
					return
				}
				// Run validation
				errors := s.validator.Validate(value, validations)
				s.validationErrors[compID] = errors

				affected := s.tracker.Affected(changed)
				// Re-render the field itself (for validation display) plus affected
				toRender := []string{compID}
				for _, id := range affected {
					if id != compID {
						toRender = append(toRender, id)
					}
				}
				s.renderComponents(toRender)
			})
			node.Callbacks["change"] = cbID
			s.trackCallback(comp.ComponentID, "change", cbID)
		}

	case protocol.CompCheckBox:
		if comp.Props.DataBinding != "" {
			binding := comp.Props.DataBinding
			compID := comp.ComponentID
			cbID := s.rend.RegisterCallback(s.id, comp.ComponentID, "toggle", func(value string) {
				boolVal := value == "true" || value == "1"
				changed, err := s.dm.Set(binding, boolVal)
				if err != nil {
					logWarn("binding", s.id, fmt.Sprintf("binding set error: %v", err))
					return
				}
				affected := s.tracker.Affected(changed)
				var toRender []string
				for _, id := range affected {
					if id != compID {
						toRender = append(toRender, id)
					}
				}
				if len(toRender) > 0 {
					s.renderComponents(toRender)
				}
			})
			node.Callbacks["toggle"] = cbID
			s.trackCallback(comp.ComponentID, "toggle", cbID)
		}

	case protocol.CompSlider:
		if comp.Props.DataBinding != "" {
			binding := comp.Props.DataBinding
			compID := comp.ComponentID
			cbID := s.rend.RegisterCallback(s.id, comp.ComponentID, "slide", func(value string) {
				var fVal float64
				fmt.Sscanf(value, "%f", &fVal)
				changed, err := s.dm.Set(binding, fVal)
				if err != nil {
					logWarn("binding", s.id, fmt.Sprintf("slider binding error: %v", err))
					return
				}
				affected := s.tracker.Affected(changed)
				var toRender []string
				for _, id := range affected {
					if id != compID {
						toRender = append(toRender, id)
					}
				}
				if len(toRender) > 0 {
					s.renderComponents(toRender)
				}
			})
			node.Callbacks["slide"] = cbID
			s.trackCallback(comp.ComponentID, "slide", cbID)
		}

	case protocol.CompChoicePicker:
		if comp.Props.DataBinding != "" {
			binding := comp.Props.DataBinding
			compID := comp.ComponentID
			cbID := s.rend.RegisterCallback(s.id, comp.ComponentID, "select", func(value string) {
				changed, err := s.dm.Set(binding, value)
				if err != nil {
					logWarn("binding", s.id, fmt.Sprintf("picker binding error: %v", err))
					return
				}
				affected := s.tracker.Affected(changed)
				var toRender []string
				for _, id := range affected {
					if id != compID {
						toRender = append(toRender, id)
					}
				}
				if len(toRender) > 0 {
					s.renderComponents(toRender)
				}
			})
			node.Callbacks["select"] = cbID
			s.trackCallback(comp.ComponentID, "select", cbID)
		}

	case protocol.CompDateTimeInput:
		if comp.Props.DataBinding != "" {
			binding := comp.Props.DataBinding
			compID := comp.ComponentID
			cbID := s.rend.RegisterCallback(s.id, comp.ComponentID, "datechange", func(value string) {
				changed, err := s.dm.Set(binding, value)
				if err != nil {
					logWarn("binding", s.id, fmt.Sprintf("date binding error: %v", err))
					return
				}
				affected := s.tracker.Affected(changed)
				var toRender []string
				for _, id := range affected {
					if id != compID {
						toRender = append(toRender, id)
					}
				}
				if len(toRender) > 0 {
					s.renderComponents(toRender)
				}
			})
			node.Callbacks["datechange"] = cbID
			s.trackCallback(comp.ComponentID, "datechange", cbID)
		}

	case protocol.CompTabs:
		if comp.Props.DataBinding != "" {
			binding := comp.Props.DataBinding
			compID := comp.ComponentID
			cbID := s.rend.RegisterCallback(s.id, comp.ComponentID, "select", func(value string) {
				changed, err := s.dm.Set(binding, value)
				if err != nil {
					logWarn("binding", s.id, fmt.Sprintf("tabs binding error: %v", err))
					return
				}
				affected := s.tracker.Affected(changed)
				var toRender []string
				for _, id := range affected {
					if id != compID {
						toRender = append(toRender, id)
					}
				}
				if len(toRender) > 0 {
					s.renderComponents(toRender)
				}
			})
			node.Callbacks["select"] = cbID
			s.trackCallback(comp.ComponentID, "select", cbID)
		}

	case protocol.CompVideo:
		if comp.Props.OnEnded != nil && comp.Props.OnEnded.Action != nil {
			action := comp.Props.OnEnded.Action
			cbID := s.rend.RegisterCallback(s.id, comp.ComponentID, "ended", func(data string) {
				if action.Event != nil {
					resolved := s.resolveDataRefs(action.Event)
					if s.ActionHandler != nil {
						s.ActionHandler(s.id, action.Event, resolved)
					}
				} else if action.FunctionCall != nil {
					s.executeFunctionCall(action.FunctionCall)
				}
			})
			node.Callbacks["ended"] = cbID
			s.trackCallback(comp.ComponentID, "ended", cbID)
		}

	case protocol.CompAudioPlayer:
		if comp.Props.OnEnded != nil && comp.Props.OnEnded.Action != nil {
			action := comp.Props.OnEnded.Action
			cbID := s.rend.RegisterCallback(s.id, comp.ComponentID, "ended", func(data string) {
				if action.Event != nil {
					resolved := s.resolveDataRefs(action.Event)
					if s.ActionHandler != nil {
						s.ActionHandler(s.id, action.Event, resolved)
					}
				} else if action.FunctionCall != nil {
					s.executeFunctionCall(action.FunctionCall)
				}
			})
			node.Callbacks["ended"] = cbID
			s.trackCallback(comp.ComponentID, "ended", cbID)
		}

	case protocol.CompSearchField:
		if comp.Props.DataBinding != "" {
			binding := comp.Props.DataBinding
			compID := comp.ComponentID
			cbID := s.rend.RegisterCallback(s.id, comp.ComponentID, "change", func(value string) {
				changed, err := s.dm.Set(binding, value)
				if err != nil {
					logWarn("binding", s.id, fmt.Sprintf("searchfield binding error: %v", err))
					return
				}
				affected := s.tracker.Affected(changed)
				var toRender []string
				for _, id := range affected {
					if id != compID {
						toRender = append(toRender, id)
					}
				}
				if len(toRender) > 0 {
					s.renderComponents(toRender)
				}
			})
			node.Callbacks["change"] = cbID
			s.trackCallback(comp.ComponentID, "change", cbID)
		}

	case protocol.CompOutlineView:
		if comp.Props.DataBinding != "" {
			binding := comp.Props.DataBinding
			compID := comp.ComponentID
			cbID := s.rend.RegisterCallback(s.id, comp.ComponentID, "select", func(value string) {
				changed, err := s.dm.Set(binding, value)
				if err != nil {
					logWarn("binding", s.id, fmt.Sprintf("outlineview binding error: %v", err))
					return
				}
				affected := s.tracker.Affected(changed)
				var toRender []string
				for _, id := range affected {
					if id != compID {
						toRender = append(toRender, id)
					}
				}
				if len(toRender) > 0 {
					s.renderComponents(toRender)
				}
			})
			node.Callbacks["select"] = cbID
			s.trackCallback(comp.ComponentID, "select", cbID)
		}

	case protocol.CompRichTextEditor:
		compID := comp.ComponentID
		binding := comp.Props.DataBinding
		onChange := comp.Props.OnChange
		if binding != "" || onChange != nil {
			cbID := s.rend.RegisterCallback(s.id, comp.ComponentID, "change", func(value string) {
				var allChanged []string
				// Write to dataBinding path if set
				if binding != "" {
					changed, err := s.dm.Set(binding, value)
					if err != nil {
						logWarn("binding", s.id, fmt.Sprintf("richtexteditor binding error: %v", err))
						return
					}
					allChanged = append(allChanged, changed...)
				}
				// Fire onChange action — write value to /_input so action can reference it
				if onChange != nil && onChange.Action != nil {
					s.dm.Set("/_input", value)
					action := onChange.Action
					if action.FunctionCall != nil {
						s.executeFunctionCall(action.FunctionCall)
					} else if action.Event != nil && s.ActionHandler != nil {
						resolved := make(map[string]interface{})
						for _, ref := range action.Event.DataRefs {
							val, _ := s.dm.Get(ref)
							resolved[ref] = val
						}
						s.ActionHandler(s.id, action.Event, resolved)
					}
					s.dm.Delete("/_input")
				}
				// Re-render affected components (excluding source editor)
				if binding != "" {
					affected := s.tracker.Affected(allChanged)
					var toRender []string
					for _, id := range affected {
						if id != compID {
							toRender = append(toRender, id)
						}
					}
					if len(toRender) > 0 {
						s.renderComponents(toRender)
					}
				}
			})
			node.Callbacks["change"] = cbID
			s.trackCallback(comp.ComponentID, "change", cbID)
		}
		if comp.Props.FormatBinding != "" {
			binding := comp.Props.FormatBinding
			cbID := s.rend.RegisterCallback(s.id, comp.ComponentID, "formatchange", func(data string) {
				var formatState map[string]interface{}
				if err := json.Unmarshal([]byte(data), &formatState); err != nil {
					logWarn("binding", s.id, fmt.Sprintf("formatchange parse error: %v", err))
					return
				}
				changed, err := s.dm.Set(binding, formatState)
				if err != nil {
					logWarn("binding", s.id, fmt.Sprintf("formatchange binding error: %v", err))
					return
				}
				affected := s.tracker.Affected(changed)
				if len(affected) > 0 {
					s.renderComponents(affected)
				}
			})
			node.Callbacks["formatchange"] = cbID
			s.trackCallback(comp.ComponentID, "formatchange", cbID)
		}

	case protocol.CompModal:
		binding := comp.Props.DataBinding
		compID := comp.ComponentID
		onDismiss := comp.Props.OnDismiss
		cbID := s.rend.RegisterCallback(s.id, comp.ComponentID, "dismiss", func(data string) {
			var allChanged []string
			if binding != "" {
				changed, err := s.dm.Set(binding, false)
				if err != nil {
					logWarn("binding", s.id, fmt.Sprintf("modal binding error: %v", err))
				} else {
					allChanged = append(allChanged, changed...)
				}
			}
			if onDismiss != nil && onDismiss.Action != nil {
				if onDismiss.Action.Event != nil {
					resolved := s.resolveDataRefs(onDismiss.Action.Event)
					if s.ActionHandler != nil {
						s.ActionHandler(s.id, onDismiss.Action.Event, resolved)
					}
				} else if onDismiss.Action.FunctionCall != nil {
					s.executeFunctionCall(onDismiss.Action.FunctionCall)
				}
			}
			affected := s.tracker.Affected(allChanged)
			var toRender []string
			for _, id := range affected {
				if id != compID {
					toRender = append(toRender, id)
				}
			}
			toRender = append(toRender, compID)
			s.renderComponents(toRender)
		})
		node.Callbacks["dismiss"] = cbID
		s.trackCallback(comp.ComponentID, "dismiss", cbID)
	}

	// Context menu support for any component type
	if comp.Props.ContextMenu != nil {
		var menuItems []protocol.MenuItem
		if err := json.Unmarshal(comp.Props.ContextMenu, &menuItems); err == nil && len(menuItems) > 0 {
			specs := s.buildContextMenuSpecs(comp.ComponentID, menuItems)
			if data, err := json.Marshal(specs); err == nil {
				node.Props.ContextMenu = string(data)
			}
		}
	}

	// General onClick support for any component type (if not already handled above)
	if _, hasClick := node.Callbacks["click"]; !hasClick {
		if comp.Props.OnClick != nil && comp.Props.OnClick.Action != nil {
			action := comp.Props.OnClick.Action
			compIDForLog := comp.ComponentID
			cbID := s.rend.RegisterCallback(s.id, comp.ComponentID, "click", func(data string) {
				jlog.Infof("click", s.id, "onClick: %s", compIDForLog)
				if action.StandardAction != "" {
					s.dispatch.RunOnMain(func() {
						s.rend.PerformAction(action.StandardAction)
					})
				} else if action.Event != nil {
					resolved := s.resolveDataRefs(action.Event)
					if s.ActionHandler != nil {
						s.ActionHandler(s.id, action.Event, resolved)
					}
				} else if action.FunctionCall != nil {
					s.executeFunctionCall(action.FunctionCall)
				}
			})
			node.Callbacks["click"] = cbID
			s.trackCallback(comp.ComponentID, "click", cbID)
		}
	}
}

func (s *Surface) trackCallback(componentID, eventType string, cbID renderer.CallbackID) {
	if s.activeCallbacks[componentID] == nil {
		s.activeCallbacks[componentID] = make(map[string]renderer.CallbackID)
	}
	s.activeCallbacks[componentID][eventType] = cbID
}

// expandComponentInstances expands useComponent references into concrete components.
// Called before expandTemplates. For each component with UseComponent set:
// 1. Looks up the definition
// 2. Substitutes params
// 3. Rewrites scoped paths ($/ prefix)
// 4. Rewrites component IDs (_root → instanceId, others → instanceId__originalId)
// 5. Parses back into protocol.Component structs
func (s *Surface) expandComponentInstances(comps []protocol.Component) []protocol.Component {
	var result []protocol.Component
	for _, comp := range comps {
		if comp.UseComponent == "" {
			result = append(result, comp)
			continue
		}

		def, ok := s.compDefs[comp.UseComponent]
		if !ok {
			logWarn("component", s.id, fmt.Sprintf("unknown component definition %q", comp.UseComponent))
			result = append(result, comp)
			continue
		}

		expanded := s.expandOneComponentInstance(comp, def)
		result = append(result, expanded...)
	}
	return result
}

func (s *Surface) expandOneComponentInstance(inst protocol.Component, def *protocol.DefineComponent) []protocol.Component {
	// Parse raw JSON components into maps
	trees, err := jsonToMaps(def.Components)
	if err != nil {
		logWarn("component", s.id, fmt.Sprintf("parse component definition %q: %v", def.Name, err))
		return []protocol.Component{inst}
	}

	// Build args map: use inst.Args, ensuring all param names map to something
	args := make(map[string]interface{}, len(def.Params))
	for _, p := range def.Params {
		if v, ok := inst.Args[p]; ok {
			args[p] = v
		}
	}

	// Substitute params in each component tree
	for i, tree := range trees {
		trees[i] = substituteParams(tree, args).(map[string]interface{})
	}

	// Determine scope
	scope := inst.Scope
	if scope == "" {
		// Check if definition uses scoped paths ($ prefix)
		if treeHasScopedPaths(trees) {
			scope = "/" + inst.ComponentID
		}
	}

	// Rewrite scoped paths if scope is set
	if scope != "" {
		for i, tree := range trees {
			trees[i] = rewriteScopedPaths(tree, scope).(map[string]interface{})
		}
	}

	// Build ID map: _root → instanceId, _X → instanceId__X
	idMap := make(map[string]string, len(trees))
	for _, tree := range trees {
		cid, _ := tree["componentId"].(string)
		if cid == "_root" {
			idMap[cid] = inst.ComponentID
		} else {
			idMap[cid] = inst.ComponentID + "_" + cid
		}
	}

	// Rewrite IDs
	rewriteComponentIDs(trees, idMap)

	// Apply parent/style from the instance to _root (now the instance ID)
	for _, tree := range trees {
		cid, _ := tree["componentId"].(string)
		if cid == inst.ComponentID {
			// Preserve parentId from instantiation
			if inst.ParentID != "" {
				tree["parentId"] = inst.ParentID
			}
			// Merge instance-level style onto definition style
			instStyleJSON, _ := json.Marshal(inst.Style)
			var instStyle map[string]interface{}
			json.Unmarshal(instStyleJSON, &instStyle)
			if len(instStyle) > 0 {
				if existing, ok := tree["style"].(map[string]interface{}); ok {
					for k, v := range instStyle {
						existing[k] = v
					}
				} else {
					tree["style"] = instStyle
				}
			}
			break
		}
	}

	// Re-serialize and parse into protocol.Component structs
	var result []protocol.Component
	for _, tree := range trees {
		data, err := json.Marshal(tree)
		if err != nil {
			logWarn("component", s.id, fmt.Sprintf("marshal expanded component: %v", err))
			continue
		}
		var comp protocol.Component
		if err := json.Unmarshal(data, &comp); err != nil {
			logWarn("component", s.id, fmt.Sprintf("unmarshal expanded component: %v", err))
			continue
		}
		result = append(result, comp)
	}

	return result
}

// treeHasScopedPaths checks if any string value in the trees starts with "$/".
func treeHasScopedPaths(trees []map[string]interface{}) bool {
	for _, tree := range trees {
		if valuHasScopedPath(tree) {
			return true
		}
	}
	return false
}

func valuHasScopedPath(val interface{}) bool {
	switch v := val.(type) {
	case map[string]interface{}:
		for _, child := range v {
			if valuHasScopedPath(child) {
				return true
			}
		}
	case []interface{}:
		for _, child := range v {
			if valuHasScopedPath(child) {
				return true
			}
		}
	case string:
		if len(v) >= 2 && v[0] == '$' && v[1] == '/' {
			return true
		}
	}
	return false
}

// expandTemplates processes components with template children, expanding them
// into static children based on data model arrays.
func (s *Surface) expandTemplates(comps []protocol.Component) []protocol.Component {
	// Index components in this batch AND pending buffer for template lookup.
	// Templates from earlier batches are in pendingComponents (not yet in tree).
	compMap := make(map[string]*protocol.Component, len(comps)+len(s.pendingComponents))
	for i := range s.pendingComponents {
		compMap[s.pendingComponents[i].ComponentID] = &s.pendingComponents[i]
	}
	for i := range comps {
		compMap[comps[i].ComponentID] = &comps[i]
	}

	var result []protocol.Component
	usedAsTemplate := make(map[string]bool)

	for i := range comps {
		comp := comps[i]
		if comp.Children == nil || comp.Children.Template == nil {
			result = append(result, comp)
			continue
		}

		tmpl := comp.Children.Template

		// Collect the full template subtree (template root + descendants)
		templateTree := s.collectTemplateTree(tmpl.TemplateID, compMap)
		if len(templateTree) == 0 {
			result = append(result, comp)
			continue
		}
		for _, tc := range templateTree {
			usedAsTemplate[tc.ComponentID] = true
		}

		// Resolve the forEach data source and base path
		var items []interface{}
		var basePath string
		if tmpl.ForEachFunc != nil {
			evaluator := NewEvaluator(s.dm)
			evaluator.FFI = s.ffi
			evaluator.Native = s.native
			evaluator.customFuncs = s.funcDefs
			val, err := evaluator.Eval(tmpl.ForEachFunc.Name, tmpl.ForEachFunc.Args)
			if err != nil {
				logWarn("template", s.id, fmt.Sprintf("forEach func error for %s: %v", comp.ComponentID, err))
				result = append(result, comp)
				continue
			}
			var ok bool
			items, ok = val.([]interface{})
			if !ok {
				result = append(result, comp)
				continue
			}
			basePath = "/_computed/" + comp.ComponentID
			s.dm.Set(basePath, items)
			// Register bindings for all paths referenced in function call args
			for _, path := range PathsInArgs(tmpl.ForEachFunc.Args) {
				s.tracker.Register(path, comp.ComponentID)
			}
		} else {
			val, found := s.dm.Get(tmpl.ForEach)
			if !found {
				result = append(result, comp)
				continue
			}
			var ok bool
			items, ok = val.([]interface{})
			if !ok {
				result = append(result, comp)
				continue
			}
			basePath = tmpl.ForEach
			s.tracker.Register(tmpl.ForEach, comp.ComponentID)
		}

		// Save state for re-expansion on data changes
		s.forEachMetas[comp.ComponentID] = &forEachMeta{
			component:    comps[i], // original component with Template children
			templateTree: templateTree,
		}

		// Generate children
		var childIDs []string
		parentPrefix := comp.ComponentID
		for idx := range items {
			itemPath := fmt.Sprintf("%s/%d", basePath, idx)

			// Clone the entire template subtree for this index
			// IDs are namespaced by parent to avoid collisions when
			// multiple forEach blocks share the same template.
			for _, tc := range templateTree {
				clone := deepCopyComponent(tc)
				clone.ComponentID = fmt.Sprintf("%s_%s_%d", parentPrefix, tc.ComponentID, idx)

				// Rewrite parent references
				if tc.ComponentID == tmpl.TemplateID {
					clone.ParentID = comp.ComponentID
					childIDs = append(childIDs, clone.ComponentID)
				} else {
					clone.ParentID = fmt.Sprintf("%s_%s_%d", parentPrefix, tc.ParentID, idx)
				}

				// Rewrite static children IDs
				if clone.Children != nil && clone.Children.Static != nil {
					newChildren := make([]string, len(clone.Children.Static))
					for j, cid := range clone.Children.Static {
						newChildren[j] = fmt.Sprintf("%s_%s_%d", parentPrefix, cid, idx)
					}
					clone.Children = &protocol.ChildList{Static: newChildren}
				}

				// Rewrite path references
				s.rewritePaths(&clone, tmpl.ItemVariable, itemPath)

				result = append(result, clone)
			}
		}

		// Replace template children with static list
		comp.Children = &protocol.ChildList{Static: childIDs}
		result = append(result, comp)
	}

	// Remove template source components from the result
	if len(usedAsTemplate) > 0 {
		filtered := result[:0]
		for _, c := range result {
			if !usedAsTemplate[c.ComponentID] {
				filtered = append(filtered, c)
			}
		}
		result = filtered
	}

	return result
}

// collectTemplateTree returns the template root component and all its descendants.
func (s *Surface) collectTemplateTree(rootID string, compMap map[string]*protocol.Component) []protocol.Component {
	var tree []protocol.Component

	// Find root
	root, ok := compMap[rootID]
	if !ok {
		tc, found := s.tree.Get(rootID)
		if !found {
			return nil
		}
		root = tc
	}
	tree = append(tree, *root)

	// BFS to collect all descendants
	queue := []string{}
	if root.Children != nil && root.Children.Static != nil {
		queue = append(queue, root.Children.Static...)
	}

	for len(queue) > 0 {
		cid := queue[0]
		queue = queue[1:]

		child, ok := compMap[cid]
		if !ok {
			tc, found := s.tree.Get(cid)
			if !found {
				continue
			}
			child = tc
		}
		tree = append(tree, *child)

		if child.Children != nil && child.Children.Static != nil {
			queue = append(queue, child.Children.Static...)
		}
	}

	return tree
}

// rewritePaths replaces /{itemVariable}/... path references with the actual array index path.
func (s *Surface) rewritePaths(comp *protocol.Component, itemVar string, itemPath string) {
	prefix := "/" + itemVar
	rewriteString := func(ds *protocol.DynamicString) {
		if ds == nil {
			return
		}
		if ds.IsPath {
			ds.Path = rewritePath(ds.Path, prefix, itemPath)
		}
		if ds.IsFunc && ds.FunctionCall != nil {
			rewriteActionArgs(ds.FunctionCall.Args, prefix, itemPath)
		}
	}
	rewriteNumber := func(dn *protocol.DynamicNumber) {
		if dn == nil {
			return
		}
		if dn.IsPath {
			dn.Path = rewritePath(dn.Path, prefix, itemPath)
		}
		if dn.IsFunc && dn.FunctionCall != nil {
			rewriteActionArgs(dn.FunctionCall.Args, prefix, itemPath)
		}
	}
	rewriteBool := func(db *protocol.DynamicBoolean) {
		if db == nil {
			return
		}
		if db.IsPath {
			db.Path = rewritePath(db.Path, prefix, itemPath)
		}
		if db.IsFunc && db.FunctionCall != nil {
			rewriteActionArgs(db.FunctionCall.Args, prefix, itemPath)
		}
	}

	p := &comp.Props
	rewriteString(p.Content)
	rewriteString(p.Title)
	rewriteString(p.Subtitle)
	rewriteString(p.Label)
	rewriteString(p.Placeholder)
	rewriteString(p.Value)
	rewriteString(p.Src)
	rewriteString(p.Alt)
	rewriteString(p.Name)
	rewriteString(p.DateValue)
	rewriteString(p.ActiveTab)
	rewriteNumber(p.Min)
	rewriteNumber(p.Max)
	rewriteNumber(p.Step)
	rewriteNumber(p.SliderValue)
	rewriteBool(p.Disabled)
	rewriteBool(p.Checked)
	rewriteBool(p.ReadOnly)
	rewriteBool(p.Collapsible)
	rewriteBool(p.Collapsed)
	rewriteBool(p.EnableDate)
	rewriteBool(p.EnableTime)
	rewriteBool(p.MutuallyExclusive)
	rewriteBool(p.Visible)
	rewriteBool(p.Autoplay)
	rewriteBool(p.Loop)
	rewriteBool(p.Controls)
	rewriteBool(p.Muted)
	rewriteBool(p.Vertical)
	rewriteBool(p.Editable)
	rewriteString(p.OutlineData)
	rewriteString(p.SelectedID)
	rewriteString(p.RichContent)

	// Rewrite dynamic style paths
	st := &comp.Style
	rewriteString(st.BackgroundColor)
	rewriteString(st.TextColor)
	rewriteString(st.FontWeight)
	rewriteString(st.TextAlign)
	rewriteString(st.FontFamily)
	rewriteNumber(st.CornerRadius)
	rewriteNumber(st.Width)
	rewriteNumber(st.Height)
	rewriteNumber(st.FontSize)
	rewriteNumber(st.Opacity)
	rewriteNumber(st.FlexGrow)

	// Rewrite data binding
	if p.DataBinding != "" {
		p.DataBinding = rewritePath(p.DataBinding, prefix, itemPath)
	}
	if p.FormatBinding != "" {
		p.FormatBinding = rewritePath(p.FormatBinding, prefix, itemPath)
	}

	// Rewrite paths in onClick action
	rewriteAction := func(ea *protocol.EventAction) {
		if ea == nil || ea.Action == nil {
			return
		}
		if ea.Action.FunctionCall != nil {
			rewriteActionArgs(ea.Action.FunctionCall.Args, prefix, itemPath)
		}
		if ea.Action.Event != nil {
			for i, ref := range ea.Action.Event.DataRefs {
				ea.Action.Event.DataRefs[i] = rewritePath(ref, prefix, itemPath)
			}
		}
	}
	rewriteAction(p.OnClick)
	rewriteAction(p.OnChange)
	rewriteAction(p.OnToggle)
	rewriteAction(p.OnSlide)
	rewriteAction(p.OnSelect)
	rewriteAction(p.OnDateChange)
	rewriteAction(p.OnDismiss)
	rewriteAction(p.OnEnded)
	rewriteAction(p.OnSearch)
	rewriteAction(p.OnRichChange)

	// Rewrite paths in contextMenu JSON
	if p.ContextMenu != nil {
		var raw interface{}
		if err := json.Unmarshal(p.ContextMenu, &raw); err == nil {
			rewriteActionArgs(raw, prefix, itemPath)
			if data, err := json.Marshal(raw); err == nil {
				p.ContextMenu = data
			}
		}
	}
}

// deepCopyComponent creates a deep copy of a component, including all pointer fields in Props.
func deepCopyComponent(c protocol.Component) protocol.Component {
	clone := c
	p := &clone.Props

	// Deep copy all DynamicString pointers (including FunctionCall args trees)
	p.Content = deepCopyDynString(p.Content)
	p.Title = deepCopyDynString(p.Title)
	p.Subtitle = deepCopyDynString(p.Subtitle)
	p.Label = deepCopyDynString(p.Label)
	p.Placeholder = deepCopyDynString(p.Placeholder)
	p.Value = deepCopyDynString(p.Value)
	p.Src = deepCopyDynString(p.Src)
	p.Alt = deepCopyDynString(p.Alt)
	p.Name = deepCopyDynString(p.Name)
	p.DateValue = deepCopyDynString(p.DateValue)
	p.ActiveTab = deepCopyDynString(p.ActiveTab)
	p.OutlineData = deepCopyDynString(p.OutlineData)
	p.SelectedID = deepCopyDynString(p.SelectedID)
	p.RichContent = deepCopyDynString(p.RichContent)

	// Deep copy DynamicNumber pointers
	p.Min = deepCopyDynNumber(p.Min)
	p.Max = deepCopyDynNumber(p.Max)
	p.Step = deepCopyDynNumber(p.Step)
	p.SliderValue = deepCopyDynNumber(p.SliderValue)

	// Deep copy DynamicStyleProps pointers
	st := &clone.Style
	st.BackgroundColor = deepCopyDynString(st.BackgroundColor)
	st.TextColor = deepCopyDynString(st.TextColor)
	st.FontWeight = deepCopyDynString(st.FontWeight)
	st.TextAlign = deepCopyDynString(st.TextAlign)
	st.FontFamily = deepCopyDynString(st.FontFamily)
	st.CornerRadius = deepCopyDynNumber(st.CornerRadius)
	st.Width = deepCopyDynNumber(st.Width)
	st.Height = deepCopyDynNumber(st.Height)
	st.FontSize = deepCopyDynNumber(st.FontSize)
	st.Opacity = deepCopyDynNumber(st.Opacity)
	st.FlexGrow = deepCopyDynNumber(st.FlexGrow)

	// Deep copy DynamicBoolean pointers
	p.Disabled = deepCopyDynBool(p.Disabled)
	p.Checked = deepCopyDynBool(p.Checked)
	p.ReadOnly = deepCopyDynBool(p.ReadOnly)
	p.Collapsible = deepCopyDynBool(p.Collapsible)
	p.Collapsed = deepCopyDynBool(p.Collapsed)
	p.EnableDate = deepCopyDynBool(p.EnableDate)
	p.EnableTime = deepCopyDynBool(p.EnableTime)
	p.MutuallyExclusive = deepCopyDynBool(p.MutuallyExclusive)
	p.Visible = deepCopyDynBool(p.Visible)
	p.Autoplay = deepCopyDynBool(p.Autoplay)
	p.Loop = deepCopyDynBool(p.Loop)
	p.Controls = deepCopyDynBool(p.Controls)
	p.Muted = deepCopyDynBool(p.Muted)
	p.Vertical = deepCopyDynBool(p.Vertical)
	p.Editable = deepCopyDynBool(p.Editable)

	// Deep copy event actions (contain mutable Args trees)
	p.OnClick = deepCopyEventAction(p.OnClick)
	p.OnChange = deepCopyEventAction(p.OnChange)
	p.OnToggle = deepCopyEventAction(p.OnToggle)
	p.OnSlide = deepCopyEventAction(p.OnSlide)
	p.OnSelect = deepCopyEventAction(p.OnSelect)
	p.OnDateChange = deepCopyEventAction(p.OnDateChange)
	p.OnDismiss = deepCopyEventAction(p.OnDismiss)
	p.OnEnded = deepCopyEventAction(p.OnEnded)
	p.OnSearch = deepCopyEventAction(p.OnSearch)
	p.OnRichChange = deepCopyEventAction(p.OnRichChange)

	// Deep copy ContextMenu raw bytes
	if p.ContextMenu != nil {
		cm := make(json.RawMessage, len(p.ContextMenu))
		copy(cm, p.ContextMenu)
		p.ContextMenu = cm
	}

	// Deep copy children
	if c.Children != nil {
		cl := *c.Children
		if cl.Static != nil {
			s := make([]string, len(cl.Static))
			copy(s, cl.Static)
			cl.Static = s
		}
		clone.Children = &cl
	}

	return clone
}

// deepCopyEventAction creates a deep copy of an EventAction, including its Args tree.
func deepCopyEventAction(ea *protocol.EventAction) *protocol.EventAction {
	if ea == nil {
		return nil
	}
	clone := *ea
	if ea.Action != nil {
		a := *ea.Action
		if a.FunctionCall != nil {
			fc := *a.FunctionCall
			fc.Args = deepCopyInterface(fc.Args)
			a.FunctionCall = &fc
		}
		if a.Event != nil {
			e := *a.Event
			if e.DataRefs != nil {
				refs := make([]string, len(e.DataRefs))
				copy(refs, e.DataRefs)
				e.DataRefs = refs
			}
			a.Event = &e
		}
		clone.Action = &a
	}
	return &clone
}

// deepCopyFuncCallArgs deep-copies a FunctionCall's []interface{} args tree.
func deepCopyFuncCallArgs(args []interface{}) []interface{} {
	if args == nil {
		return nil
	}
	out := make([]interface{}, len(args))
	for i, a := range args {
		out[i] = deepCopyInterface(a)
	}
	return out
}

// deepCopyDynString deep-copies a DynamicString including its FunctionCall args tree.
func deepCopyDynString(ds *protocol.DynamicString) *protocol.DynamicString {
	if ds == nil {
		return nil
	}
	v := *ds
	if v.FunctionCall != nil {
		fc := *v.FunctionCall
		fc.Args = deepCopyFuncCallArgs(fc.Args)
		v.FunctionCall = &fc
	}
	return &v
}

// deepCopyDynNumber deep-copies a DynamicNumber including its FunctionCall args tree.
func deepCopyDynNumber(dn *protocol.DynamicNumber) *protocol.DynamicNumber {
	if dn == nil {
		return nil
	}
	v := *dn
	if v.FunctionCall != nil {
		fc := *v.FunctionCall
		fc.Args = deepCopyFuncCallArgs(fc.Args)
		v.FunctionCall = &fc
	}
	return &v
}

// deepCopyDynBool deep-copies a DynamicBoolean including its FunctionCall args tree.
func deepCopyDynBool(db *protocol.DynamicBoolean) *protocol.DynamicBoolean {
	if db == nil {
		return nil
	}
	v := *db
	if v.FunctionCall != nil {
		fc := *v.FunctionCall
		fc.Args = deepCopyFuncCallArgs(fc.Args)
		v.FunctionCall = &fc
	}
	return &v
}

// deepCopyInterface deep-copies a JSON-like interface{} tree (maps and slices).
func deepCopyInterface(v interface{}) interface{} {
	switch val := v.(type) {
	case map[string]interface{}:
		m := make(map[string]interface{}, len(val))
		for k, v := range val {
			m[k] = deepCopyInterface(v)
		}
		return m
	case []interface{}:
		s := make([]interface{}, len(val))
		for i, v := range val {
			s[i] = deepCopyInterface(v)
		}
		return s
	default:
		return v
	}
}

// rewriteActionArgs recursively walks a functionCall's args tree and rewrites
// path strings that match the forEach item variable prefix.
func rewriteActionArgs(args interface{}, prefix, replacement string) {
	switch v := args.(type) {
	case map[string]interface{}:
		for key, val := range v {
			if key == "path" {
				if s, ok := val.(string); ok {
					v[key] = rewritePath(s, prefix, replacement)
				}
			} else {
				rewriteActionArgs(val, prefix, replacement)
			}
		}
	case []interface{}:
		for _, item := range v {
			rewriteActionArgs(item, prefix, replacement)
		}
	}
}

func rewritePath(path, prefix, replacement string) string {
	if path == prefix {
		return replacement
	}
	if len(path) > len(prefix) && path[:len(prefix)+1] == prefix+"/" {
		return replacement + path[len(prefix):]
	}
	return path
}

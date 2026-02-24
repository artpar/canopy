package engine

import (
	"encoding/json"
	"fmt"
	"jview/protocol"
	"jview/renderer"
	"log"
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
	assets    *AssetRegistry

	// activeCallbacks tracks registered callbacks: componentID → eventType → CallbackID
	activeCallbacks map[string]map[string]renderer.CallbackID

	// validationErrors tracks current validation errors: componentID → []errorMessages
	validationErrors map[string][]string

	// funcDefs holds user-defined functions for the evaluator
	funcDefs map[string]*FuncDef

	// compDefs holds user-defined component templates
	compDefs map[string]*protocol.DefineComponent

	// ActionHandler is called when a component triggers a server-bound event.
	ActionHandler func(surfaceID string, event *protocol.EventDef, data map[string]interface{})
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

// HandleUpdateComponents processes a batch of component definitions.
func (s *Surface) HandleUpdateComponents(msg protocol.UpdateComponents) {
	comps := s.expandComponentInstances(msg.Components)
	expanded := s.expandTemplates(comps)
	changed := s.tree.Update(expanded)
	s.renderComponents(changed)
}

// HandleUpdateDataModel applies data model operations and re-renders affected components.
func (s *Surface) HandleUpdateDataModel(msg protocol.UpdateDataModel) {
	var allChanged []string
	for _, op := range msg.Ops {
		var changed []string
		var err error
		switch op.Op {
		case "add", "replace":
			changed, err = s.dm.Set(op.Path, op.Value)
		case "remove":
			changed, err = s.dm.Delete(op.Path)
		default:
			log.Printf("surface %s: unknown data op %q", s.id, op.Op)
			continue
		}
		if err != nil {
			log.Printf("surface %s: data op error: %v", s.id, err)
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

// renderComponents resolves and dispatches render operations for the given component IDs.
func (s *Surface) renderComponents(componentIDs []string) {
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
			handle := s.rend.GetHandle(s.id, w.node.ComponentID)
			if handle == 0 {
				s.rend.CreateView(s.id, w.node)
			} else {
				s.rend.UpdateView(s.id, handle, w.node)
			}
		}

		// Second pass: set children for containers (now all children exist)
		for _, w := range ordered {
			if len(w.node.ChildIDs) == 0 {
				continue
			}
			parentHandle := s.rend.GetHandle(s.id, w.node.ComponentID)
			if parentHandle == 0 {
				continue
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
	switch fc.Call {
	case "updateDataModel":
		s.executeUpdateDataModel(fc.Args)
	default:
		log.Printf("surface %s: unknown functionCall: %s", s.id, fc.Call)
	}
}

// executeUpdateDataModel applies data model operations from a functionCall's args.
// Args is expected to be map[string]interface{} with an "ops" key containing an array of ops.
// Each op has {op, path, value} where value can be a dynamic (functionCall/path ref).
func (s *Surface) executeUpdateDataModel(args interface{}) {
	argsMap, ok := args.(map[string]interface{})
	if !ok {
		log.Printf("surface %s: updateDataModel args not a map", s.id)
		return
	}
	opsRaw, ok := argsMap["ops"]
	if !ok {
		log.Printf("surface %s: updateDataModel missing ops", s.id)
		return
	}
	ops, ok := opsRaw.([]interface{})
	if !ok {
		log.Printf("surface %s: updateDataModel ops not an array", s.id)
		return
	}

	evaluator := NewEvaluator(s.dm)
	evaluator.FFI = s.ffi
	evaluator.customFuncs = s.funcDefs
	var allChanged []string

	for _, opRaw := range ops {
		opMap, ok := opRaw.(map[string]interface{})
		if !ok {
			continue
		}
		opType, _ := opMap["op"].(string)
		path, _ := opMap["path"].(string)
		if opType == "" || path == "" {
			continue
		}

		switch opType {
		case "add", "replace":
			value, err := evaluator.resolveArg(opMap["value"])
			if err != nil {
				log.Printf("surface %s: resolve value error: %v", s.id, err)
				continue
			}
			changed, err := s.dm.Set(path, value)
			if err != nil {
				log.Printf("surface %s: data op error: %v", s.id, err)
				continue
			}
			allChanged = append(allChanged, changed...)
		case "remove":
			changed, err := s.dm.Delete(path)
			if err != nil {
				log.Printf("surface %s: data op error: %v", s.id, err)
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
				if action.Event != nil {
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
					log.Printf("surface %s: binding set error: %v", s.id, err)
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
					log.Printf("surface %s: binding set error: %v", s.id, err)
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
					log.Printf("surface %s: slider binding error: %v", s.id, err)
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
					log.Printf("surface %s: picker binding error: %v", s.id, err)
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
					log.Printf("surface %s: date binding error: %v", s.id, err)
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
					log.Printf("surface %s: tabs binding error: %v", s.id, err)
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
			log.Printf("surface %s: unknown component definition %q", s.id, comp.UseComponent)
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
		log.Printf("surface %s: parse component definition %q: %v", s.id, def.Name, err)
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
			if inst.Style != (protocol.StyleProps{}) {
				instStyleJSON, _ := json.Marshal(inst.Style)
				var instStyle map[string]interface{}
				json.Unmarshal(instStyleJSON, &instStyle)
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
			log.Printf("surface %s: marshal expanded component: %v", s.id, err)
			continue
		}
		var comp protocol.Component
		if err := json.Unmarshal(data, &comp); err != nil {
			log.Printf("surface %s: unmarshal expanded component: %v", s.id, err)
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
	// Index components in this batch by ID for template lookup
	compMap := make(map[string]*protocol.Component, len(comps))
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

		// Look up the forEach array in the data model
		val, found := s.dm.Get(tmpl.ForEach)
		if !found {
			result = append(result, comp)
			continue
		}
		items, ok := val.([]interface{})
		if !ok {
			result = append(result, comp)
			continue
		}

		// Register binding on the forEach path to this parent component
		s.tracker.Register(tmpl.ForEach, comp.ComponentID)

		// Generate children
		var childIDs []string
		parentPrefix := comp.ComponentID
		for idx := range items {
			itemPath := fmt.Sprintf("%s/%d", tmpl.ForEach, idx)

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
		if ds != nil && ds.IsPath {
			ds.Path = rewritePath(ds.Path, prefix, itemPath)
		}
	}
	rewriteNumber := func(dn *protocol.DynamicNumber) {
		if dn != nil && dn.IsPath {
			dn.Path = rewritePath(dn.Path, prefix, itemPath)
		}
	}
	rewriteBool := func(db *protocol.DynamicBoolean) {
		if db != nil && db.IsPath {
			db.Path = rewritePath(db.Path, prefix, itemPath)
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

	// Rewrite data binding
	if p.DataBinding != "" {
		p.DataBinding = rewritePath(p.DataBinding, prefix, itemPath)
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
}

// deepCopyComponent creates a deep copy of a component, including all pointer fields in Props.
func deepCopyComponent(c protocol.Component) protocol.Component {
	clone := c
	p := &clone.Props

	// Deep copy all DynamicString pointers
	if p.Content != nil {
		v := *p.Content
		p.Content = &v
	}
	if p.Title != nil {
		v := *p.Title
		p.Title = &v
	}
	if p.Subtitle != nil {
		v := *p.Subtitle
		p.Subtitle = &v
	}
	if p.Label != nil {
		v := *p.Label
		p.Label = &v
	}
	if p.Placeholder != nil {
		v := *p.Placeholder
		p.Placeholder = &v
	}
	if p.Value != nil {
		v := *p.Value
		p.Value = &v
	}
	if p.Src != nil {
		v := *p.Src
		p.Src = &v
	}
	if p.Alt != nil {
		v := *p.Alt
		p.Alt = &v
	}
	if p.Name != nil {
		v := *p.Name
		p.Name = &v
	}
	if p.DateValue != nil {
		v := *p.DateValue
		p.DateValue = &v
	}
	if p.ActiveTab != nil {
		v := *p.ActiveTab
		p.ActiveTab = &v
	}

	// Deep copy DynamicNumber pointers
	if p.Min != nil {
		v := *p.Min
		p.Min = &v
	}
	if p.Max != nil {
		v := *p.Max
		p.Max = &v
	}
	if p.Step != nil {
		v := *p.Step
		p.Step = &v
	}
	if p.SliderValue != nil {
		v := *p.SliderValue
		p.SliderValue = &v
	}

	// Deep copy DynamicBoolean pointers
	if p.Disabled != nil {
		v := *p.Disabled
		p.Disabled = &v
	}
	if p.Checked != nil {
		v := *p.Checked
		p.Checked = &v
	}
	if p.ReadOnly != nil {
		v := *p.ReadOnly
		p.ReadOnly = &v
	}
	if p.Collapsible != nil {
		v := *p.Collapsible
		p.Collapsible = &v
	}
	if p.Collapsed != nil {
		v := *p.Collapsed
		p.Collapsed = &v
	}
	if p.EnableDate != nil {
		v := *p.EnableDate
		p.EnableDate = &v
	}
	if p.EnableTime != nil {
		v := *p.EnableTime
		p.EnableTime = &v
	}
	if p.MutuallyExclusive != nil {
		v := *p.MutuallyExclusive
		p.MutuallyExclusive = &v
	}
	if p.Visible != nil {
		v := *p.Visible
		p.Visible = &v
	}

	// Deep copy event actions (contain mutable Args trees)
	p.OnClick = deepCopyEventAction(p.OnClick)
	p.OnChange = deepCopyEventAction(p.OnChange)
	p.OnToggle = deepCopyEventAction(p.OnToggle)
	p.OnSlide = deepCopyEventAction(p.OnSlide)
	p.OnSelect = deepCopyEventAction(p.OnSelect)
	p.OnDateChange = deepCopyEventAction(p.OnDateChange)
	p.OnDismiss = deepCopyEventAction(p.OnDismiss)

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

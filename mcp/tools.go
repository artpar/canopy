package mcp

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"jview/jlog"
	"os"
	"time"
)

func (s *Server) registerTools() {
	s.registerListSurfaces()
	s.registerGetTree()
	s.registerGetComponent()
	s.registerGetDataModel()
	s.registerGetLayout()
	s.registerGetStyle()
	s.registerTakeScreenshot()
	s.registerPerformAction()
	s.registerClick()
	s.registerFill()
	s.registerToggle()
	s.registerInteract()
	s.registerSetDataModel()
	s.registerWaitFor()
	s.registerSendMessage()
	s.registerGetLogs()
	s.registerGetPendingActions()
	s.registerListProcesses()
	s.registerCreateProcess()
	s.registerStopProcess()
	s.registerSendToProcess()
	s.registerListChannels()
	s.registerCreateChannel()
	s.registerDeleteChannel()
	s.registerPublish()
	s.registerSubscribe()
	s.registerUnsubscribe()
	s.registerSystemTools()
}

// --- Query tools ---

func (s *Server) registerListSurfaces() {
	s.register("list_surfaces", "List all open surfaces (windows)", json.RawMessage(`{
		"type": "object",
		"properties": {},
		"additionalProperties": false
	}`), func(args json.RawMessage) *ToolCallResult {
		ids := s.sess.SurfaceIDs()
		type surfaceInfo struct {
			ID string `json:"id"`
		}
		result := make([]surfaceInfo, len(ids))
		for i, id := range ids {
			result[i] = surfaceInfo{ID: id}
		}
		cb, err := JSONContent(result)
		if err != nil {
			return &ToolCallResult{Content: []ContentBlock{ErrorContent(err.Error())}, IsError: true}
		}
		return &ToolCallResult{Content: []ContentBlock{cb}}
	})
}

func (s *Server) registerGetTree() {
	s.register("get_tree", "Get component tree for a surface", json.RawMessage(`{
		"type": "object",
		"properties": {
			"surface_id": {"type": "string", "description": "Surface ID"}
		},
		"required": ["surface_id"],
		"additionalProperties": false
	}`), func(args json.RawMessage) *ToolCallResult {
		var p struct {
			SurfaceID string `json:"surface_id"`
		}
		if err := json.Unmarshal(args, &p); err != nil {
			return errorResult("invalid params: " + err.Error())
		}
		surf := s.sess.GetSurface(p.SurfaceID)
		if surf == nil {
			return errorResult("surface not found: " + p.SurfaceID)
		}
		tree := surf.Tree()
		roots := tree.RootIDs()

		type treeNode struct {
			ID       string     `json:"id"`
			Type     string     `json:"type"`
			Children []treeNode `json:"children,omitempty"`
		}
		var buildNode func(id string) treeNode
		buildNode = func(id string) treeNode {
			comp, ok := tree.Get(id)
			if !ok {
				return treeNode{ID: id}
			}
			node := treeNode{ID: id, Type: string(comp.Type)}
			for _, childID := range tree.Children(id) {
				node.Children = append(node.Children, buildNode(childID))
			}
			return node
		}

		var rootNodes []treeNode
		for _, rid := range roots {
			rootNodes = append(rootNodes, buildNode(rid))
		}
		cb, err := JSONContent(rootNodes)
		if err != nil {
			return errorResult(err.Error())
		}
		return &ToolCallResult{Content: []ContentBlock{cb}}
	})
}

func (s *Server) registerGetComponent() {
	s.register("get_component", "Get full resolved props, layout, and style for a component", json.RawMessage(`{
		"type": "object",
		"properties": {
			"surface_id": {"type": "string", "description": "Surface ID"},
			"component_id": {"type": "string", "description": "Component ID"}
		},
		"required": ["surface_id", "component_id"],
		"additionalProperties": false
	}`), func(args json.RawMessage) *ToolCallResult {
		var p struct {
			SurfaceID   string `json:"surface_id"`
			ComponentID string `json:"component_id"`
		}
		if err := json.Unmarshal(args, &p); err != nil {
			return errorResult("invalid params: " + err.Error())
		}
		surf := s.sess.GetSurface(p.SurfaceID)
		if surf == nil {
			return errorResult("surface not found: " + p.SurfaceID)
		}
		comp, ok := surf.Tree().Get(p.ComponentID)
		if !ok {
			return errorResult("component not found: " + p.ComponentID)
		}
		node := surf.Resolver().Resolve(comp)

		layout := dispatchSync(s.disp, func() map[string]any {
			l := s.rend.QueryLayout(p.SurfaceID, p.ComponentID)
			return map[string]any{"x": l.X, "y": l.Y, "width": l.Width, "height": l.Height}
		})
		style := dispatchSync(s.disp, func() map[string]any {
			st := s.rend.QueryStyle(p.SurfaceID, p.ComponentID)
			return map[string]any{
				"fontName": st.FontName, "fontSize": st.FontSize,
				"bold": st.Bold, "italic": st.Italic,
				"textColor": st.TextColor, "bgColor": st.BgColor,
				"hidden": st.Hidden, "opacity": st.Opacity,
			}
		})

		result := map[string]any{
			"id":       node.ComponentID,
			"type":     node.Type,
			"props":    node.Props,
			"style":    node.Style,
			"children": node.ChildIDs,
			"layout":   layout,
			"computed": style,
		}
		cb, err := JSONContent(result)
		if err != nil {
			return errorResult(err.Error())
		}
		return &ToolCallResult{Content: []ContentBlock{cb}}
	})
}

func (s *Server) registerGetDataModel() {
	s.register("get_data_model", "Read value at JSON Pointer path from the data model", json.RawMessage(`{
		"type": "object",
		"properties": {
			"surface_id": {"type": "string", "description": "Surface ID"},
			"path": {"type": "string", "description": "JSON Pointer path (e.g. /users/0/name). Empty string or / returns root."}
		},
		"required": ["surface_id"],
		"additionalProperties": false
	}`), func(args json.RawMessage) *ToolCallResult {
		var p struct {
			SurfaceID string `json:"surface_id"`
			Path      string `json:"path"`
		}
		if err := json.Unmarshal(args, &p); err != nil {
			return errorResult("invalid params: " + err.Error())
		}
		surf := s.sess.GetSurface(p.SurfaceID)
		if surf == nil {
			return errorResult("surface not found: " + p.SurfaceID)
		}
		val, ok := surf.DM().Get(p.Path)
		if !ok {
			return errorResult("path not found: " + p.Path)
		}
		cb, err := JSONContent(val)
		if err != nil {
			return errorResult(err.Error())
		}
		return &ToolCallResult{Content: []ContentBlock{cb}}
	})
}

func (s *Server) registerGetLayout() {
	s.register("get_layout", "Get computed frame (x, y, width, height) for a component", json.RawMessage(`{
		"type": "object",
		"properties": {
			"surface_id": {"type": "string", "description": "Surface ID"},
			"component_id": {"type": "string", "description": "Component ID"}
		},
		"required": ["surface_id", "component_id"],
		"additionalProperties": false
	}`), func(args json.RawMessage) *ToolCallResult {
		var p struct {
			SurfaceID   string `json:"surface_id"`
			ComponentID string `json:"component_id"`
		}
		if err := json.Unmarshal(args, &p); err != nil {
			return errorResult("invalid params: " + err.Error())
		}
		layout := dispatchSync(s.disp, func() map[string]any {
			l := s.rend.QueryLayout(p.SurfaceID, p.ComponentID)
			return map[string]any{"x": l.X, "y": l.Y, "width": l.Width, "height": l.Height}
		})
		cb, err := JSONContent(layout)
		if err != nil {
			return errorResult(err.Error())
		}
		return &ToolCallResult{Content: []ContentBlock{cb}}
	})
}

func (s *Server) registerGetStyle() {
	s.register("get_style", "Get computed style (font, color, etc.) for a component", json.RawMessage(`{
		"type": "object",
		"properties": {
			"surface_id": {"type": "string", "description": "Surface ID"},
			"component_id": {"type": "string", "description": "Component ID"}
		},
		"required": ["surface_id", "component_id"],
		"additionalProperties": false
	}`), func(args json.RawMessage) *ToolCallResult {
		var p struct {
			SurfaceID   string `json:"surface_id"`
			ComponentID string `json:"component_id"`
		}
		if err := json.Unmarshal(args, &p); err != nil {
			return errorResult("invalid params: " + err.Error())
		}
		style := dispatchSync(s.disp, func() map[string]any {
			st := s.rend.QueryStyle(p.SurfaceID, p.ComponentID)
			return map[string]any{
				"fontName": st.FontName, "fontSize": st.FontSize,
				"bold": st.Bold, "italic": st.Italic,
				"textColor": st.TextColor, "bgColor": st.BgColor,
				"hidden": st.Hidden, "opacity": st.Opacity,
			}
		})
		cb, err := JSONContent(style)
		if err != nil {
			return errorResult(err.Error())
		}
		return &ToolCallResult{Content: []ContentBlock{cb}}
	})
}

func (s *Server) registerTakeScreenshot() {
	s.register("take_screenshot", "Capture window as PNG. If filePath is given, saves to disk; otherwise returns base64.", json.RawMessage(`{
		"type": "object",
		"properties": {
			"surface_id": {"type": "string", "description": "Surface ID"},
			"filePath": {"type": "string", "description": "Optional file path to save the PNG to disk"}
		},
		"required": ["surface_id"],
		"additionalProperties": false
	}`), func(args json.RawMessage) *ToolCallResult {
		var p struct {
			SurfaceID string `json:"surface_id"`
			FilePath  string `json:"filePath"`
		}
		if err := json.Unmarshal(args, &p); err != nil {
			return errorResult("invalid params: " + err.Error())
		}
		pngData := dispatchSync(s.disp, func() []byte {
			data, err := s.rend.CaptureWindow(p.SurfaceID)
			if err != nil {
				return nil
			}
			return data
		})
		if pngData == nil {
			return errorResult("screenshot capture failed for surface: " + p.SurfaceID)
		}
		if p.FilePath != "" {
			if err := os.WriteFile(p.FilePath, pngData, 0644); err != nil {
				return errorResult("failed to write screenshot: " + err.Error())
			}
			return &ToolCallResult{Content: []ContentBlock{TextContent("saved: " + p.FilePath)}}
		}
		b64 := base64.StdEncoding.EncodeToString(pngData)
		return &ToolCallResult{Content: []ContentBlock{ImageContent(b64, "image/png")}}
	})
}

func (s *Server) registerPerformAction() {
	s.register("perform_action", "Send an AppKit selector through the responder chain (e.g. selectAll:, toggleBoldface:, copy:, paste:)", json.RawMessage(`{
		"type": "object",
		"properties": {
			"selector": {"type": "string", "description": "ObjC selector name (e.g. selectAll:, toggleBoldface:, copy:)"}
		},
		"required": ["selector"],
		"additionalProperties": false
	}`), func(args json.RawMessage) *ToolCallResult {
		var p struct {
			Selector string `json:"selector"`
		}
		if err := json.Unmarshal(args, &p); err != nil {
			return errorResult("invalid params: " + err.Error())
		}
		dispatchSync(s.disp, func() struct{} {
			s.rend.PerformAction(p.Selector)
			return struct{}{}
		})
		dispatchSync(s.disp, func() struct{} { return struct{}{} })
		return &ToolCallResult{Content: []ContentBlock{TextContent("performed: " + p.Selector)}}
	})
}

// --- Interaction tools ---

func (s *Server) registerClick() {
	s.register("click", "Click a button component", json.RawMessage(`{
		"type": "object",
		"properties": {
			"surface_id": {"type": "string", "description": "Surface ID"},
			"component_id": {"type": "string", "description": "Component ID of the button"}
		},
		"required": ["surface_id", "component_id"],
		"additionalProperties": false
	}`), func(args json.RawMessage) *ToolCallResult {
		var p struct {
			SurfaceID   string `json:"surface_id"`
			ComponentID string `json:"component_id"`
		}
		if err := json.Unmarshal(args, &p); err != nil {
			return errorResult("invalid params: " + err.Error())
		}
		dispatchSync(s.disp, func() struct{} {
			s.rend.InvokeCallback(p.SurfaceID, p.ComponentID, "click", "")
			return struct{}{}
		})
		// Flush: callback may have queued renders via dispatch_async
		dispatchSync(s.disp, func() struct{} { return struct{}{} })
		return &ToolCallResult{Content: []ContentBlock{TextContent("clicked: " + p.ComponentID)}}
	})
}

func (s *Server) registerFill() {
	s.register("fill", "Type into a text field", json.RawMessage(`{
		"type": "object",
		"properties": {
			"surface_id": {"type": "string", "description": "Surface ID"},
			"component_id": {"type": "string", "description": "Component ID of the text field"},
			"value": {"type": "string", "description": "Text to type"}
		},
		"required": ["surface_id", "component_id", "value"],
		"additionalProperties": false
	}`), func(args json.RawMessage) *ToolCallResult {
		var p struct {
			SurfaceID   string `json:"surface_id"`
			ComponentID string `json:"component_id"`
			Value       string `json:"value"`
		}
		if err := json.Unmarshal(args, &p); err != nil {
			return errorResult("invalid params: " + err.Error())
		}
		dispatchSync(s.disp, func() struct{} {
			s.rend.InvokeCallback(p.SurfaceID, p.ComponentID, "change", p.Value)
			return struct{}{}
		})
		dispatchSync(s.disp, func() struct{} { return struct{}{} })
		return &ToolCallResult{Content: []ContentBlock{TextContent(fmt.Sprintf("filled %s with %q", p.ComponentID, p.Value))}}
	})
}

func (s *Server) registerToggle() {
	s.register("toggle", "Toggle a checkbox", json.RawMessage(`{
		"type": "object",
		"properties": {
			"surface_id": {"type": "string", "description": "Surface ID"},
			"component_id": {"type": "string", "description": "Component ID of the checkbox"},
			"checked": {"type": "boolean", "description": "Desired state (true/false)"}
		},
		"required": ["surface_id", "component_id", "checked"],
		"additionalProperties": false
	}`), func(args json.RawMessage) *ToolCallResult {
		var p struct {
			SurfaceID   string `json:"surface_id"`
			ComponentID string `json:"component_id"`
			Checked     bool   `json:"checked"`
		}
		if err := json.Unmarshal(args, &p); err != nil {
			return errorResult("invalid params: " + err.Error())
		}
		val := "false"
		if p.Checked {
			val = "true"
		}
		dispatchSync(s.disp, func() struct{} {
			s.rend.InvokeCallback(p.SurfaceID, p.ComponentID, "toggle", val)
			return struct{}{}
		})
		dispatchSync(s.disp, func() struct{} { return struct{}{} })
		return &ToolCallResult{Content: []ContentBlock{TextContent(fmt.Sprintf("toggled %s to %s", p.ComponentID, val))}}
	})
}

func (s *Server) registerInteract() {
	s.register("interact", "Generic interaction: slide, select, datechange", json.RawMessage(`{
		"type": "object",
		"properties": {
			"surface_id": {"type": "string", "description": "Surface ID"},
			"component_id": {"type": "string", "description": "Component ID"},
			"event": {"type": "string", "description": "Event type: slide, select, datechange"},
			"value": {"type": "string", "description": "Event value"}
		},
		"required": ["surface_id", "component_id", "event", "value"],
		"additionalProperties": false
	}`), func(args json.RawMessage) *ToolCallResult {
		var p struct {
			SurfaceID   string `json:"surface_id"`
			ComponentID string `json:"component_id"`
			Event       string `json:"event"`
			Value       string `json:"value"`
		}
		if err := json.Unmarshal(args, &p); err != nil {
			return errorResult("invalid params: " + err.Error())
		}
		dispatchSync(s.disp, func() struct{} {
			s.rend.InvokeCallback(p.SurfaceID, p.ComponentID, p.Event, p.Value)
			return struct{}{}
		})
		dispatchSync(s.disp, func() struct{} { return struct{}{} })
		return &ToolCallResult{Content: []ContentBlock{TextContent(fmt.Sprintf("sent %s to %s with value %q", p.Event, p.ComponentID, p.Value))}}
	})
}

// --- Data tools ---

func (s *Server) registerSetDataModel() {
	s.register("set_data_model", "Write value at JSON Pointer path in the data model", json.RawMessage(`{
		"type": "object",
		"properties": {
			"surface_id": {"type": "string", "description": "Surface ID"},
			"path": {"type": "string", "description": "JSON Pointer path"},
			"value": {"description": "Value to set (any JSON type)"}
		},
		"required": ["surface_id", "path", "value"],
		"additionalProperties": false
	}`), func(args json.RawMessage) *ToolCallResult {
		var p struct {
			SurfaceID string          `json:"surface_id"`
			Path      string          `json:"path"`
			Value     json.RawMessage `json:"value"`
		}
		if err := json.Unmarshal(args, &p); err != nil {
			return errorResult("invalid params: " + err.Error())
		}
		surf := s.sess.GetSurface(p.SurfaceID)
		if surf == nil {
			return errorResult("surface not found: " + p.SurfaceID)
		}
		var val any
		if err := json.Unmarshal(p.Value, &val); err != nil {
			return errorResult("invalid value: " + err.Error())
		}
		changed, err := surf.DM().Set(p.Path, val)
		if err != nil {
			return errorResult("set error: " + err.Error())
		}
		cb, _ := JSONContent(map[string]any{"changed": changed})
		return &ToolCallResult{Content: []ContentBlock{cb}}
	})
}

func (s *Server) registerWaitFor() {
	s.register("wait_for", "Poll until a component exists or data matches", json.RawMessage(`{
		"type": "object",
		"properties": {
			"surface_id": {"type": "string", "description": "Surface ID"},
			"component_id": {"type": "string", "description": "Component ID to wait for (optional)"},
			"path": {"type": "string", "description": "Data model path to check (optional)"},
			"value": {"description": "Expected value at path (optional)"},
			"timeout_ms": {"type": "integer", "description": "Max wait time in ms (default 5000)"}
		},
		"required": ["surface_id"],
		"additionalProperties": false
	}`), func(args json.RawMessage) *ToolCallResult {
		var p struct {
			SurfaceID   string          `json:"surface_id"`
			ComponentID string          `json:"component_id"`
			Path        string          `json:"path"`
			Value       json.RawMessage `json:"value"`
			TimeoutMS   int             `json:"timeout_ms"`
		}
		if err := json.Unmarshal(args, &p); err != nil {
			return errorResult("invalid params: " + err.Error())
		}
		if p.TimeoutMS <= 0 {
			p.TimeoutMS = 5000
		}

		var expectedVal any
		if len(p.Value) > 0 {
			json.Unmarshal(p.Value, &expectedVal)
		}

		deadline := time.After(time.Duration(p.TimeoutMS) * time.Millisecond)
		ticker := time.NewTicker(50 * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-deadline:
				return errorResult("timeout waiting for condition")
			case <-ticker.C:
				surf := s.sess.GetSurface(p.SurfaceID)
				if surf == nil {
					continue
				}

				if p.ComponentID != "" {
					if _, ok := surf.Tree().Get(p.ComponentID); !ok {
						continue
					}
				}

				if p.Path != "" && expectedVal != nil {
					val, ok := surf.DM().Get(p.Path)
					if !ok {
						continue
					}
					// Compare JSON representations
					a, _ := json.Marshal(val)
					b, _ := json.Marshal(expectedVal)
					if string(a) != string(b) {
						continue
					}
				}

				return &ToolCallResult{Content: []ContentBlock{TextContent("condition met")}}
			}
		}
	})
}

// --- Transport tool ---

func (s *Server) registerSendMessage() {
	s.register("send_message", "Send an A2UI JSONL message to the session (create surfaces, update components, update data model, etc.)", json.RawMessage(`{
		"type": "object",
		"properties": {
			"message": {"description": "A2UI JSONL message object (e.g. {\"type\":\"createSurface\",\"surfaceId\":\"main\",\"title\":\"My App\"})"}
		},
		"required": ["message"],
		"additionalProperties": false
	}`), func(args json.RawMessage) *ToolCallResult {
		var p struct {
			Message json.RawMessage `json:"message"`
		}
		if err := json.Unmarshal(args, &p); err != nil {
			return errorResult("invalid params: " + err.Error())
		}
		if err := s.SendMessage(p.Message); err != nil {
			return errorResult("parse error: " + err.Error())
		}
		return &ToolCallResult{Content: []ContentBlock{TextContent("message sent")}}
	})
}

// --- Log query tool ---

func (s *Server) registerGetLogs() {
	s.register("get_logs", "Query application logs with filtering and pagination", json.RawMessage(`{
		"type": "object",
		"properties": {
			"level": {"type": "string", "description": "Minimum level filter: debug, info, warn, error (default info)"},
			"component": {"type": "string", "description": "Filter by component name (e.g. session, transport, darwin, mcp)"},
			"surface_id": {"type": "string", "description": "Filter by surface ID"},
			"pattern": {"type": "string", "description": "Regex pattern to match against message text"},
			"limit": {"type": "integer", "description": "Max entries to return (default 50, max 500)"},
			"offset": {"type": "integer", "description": "Skip first N matching entries (default 0)"}
		},
		"additionalProperties": false
	}`), func(args json.RawMessage) *ToolCallResult {
		var p struct {
			Level     string `json:"level"`
			Component string `json:"component"`
			SurfaceID string `json:"surface_id"`
			Pattern   string `json:"pattern"`
			Limit     int    `json:"limit"`
			Offset    int    `json:"offset"`
		}
		if err := json.Unmarshal(args, &p); err != nil {
			return errorResult("invalid params: " + err.Error())
		}

		minLevel := jlog.LevelInfo
		if p.Level != "" {
			minLevel = jlog.ParseLevel(p.Level)
		}

		opts := jlog.QueryOpts{
			MinLevel:  minLevel,
			Component: p.Component,
			Surface:   p.SurfaceID,
			Pattern:   p.Pattern,
			Limit:     p.Limit,
			Offset:    p.Offset,
		}

		result := jlog.Query(opts)
		cb, err := JSONContent(result)
		if err != nil {
			return errorResult(err.Error())
		}
		return &ToolCallResult{Content: []ContentBlock{cb}}
	})
}

// --- Action queue tool ---

func (s *Server) registerGetPendingActions() {
	s.register("get_pending_actions", "Poll for pending user actions (clicks, inputs). Returns immediately if actions are queued, or waits up to timeout_ms for new actions.", json.RawMessage(`{
		"type": "object",
		"properties": {
			"timeout_ms": {"type": "integer", "description": "Max wait time in ms for new actions (default 0 = no wait)"}
		},
		"additionalProperties": false
	}`), func(args json.RawMessage) *ToolCallResult {
		var p struct {
			TimeoutMS int `json:"timeout_ms"`
		}
		if err := json.Unmarshal(args, &p); err != nil {
			return errorResult("invalid params: " + err.Error())
		}

		// Try immediate drain first
		actions := s.DrainActions()
		if len(actions) > 0 {
			cb, err := JSONContent(actions)
			if err != nil {
				return errorResult(err.Error())
			}
			return &ToolCallResult{Content: []ContentBlock{cb}}
		}

		// If timeout requested, wait for signal
		if p.TimeoutMS > 0 {
			select {
			case <-s.actionCh:
			case <-time.After(time.Duration(p.TimeoutMS) * time.Millisecond):
			}
			actions = s.DrainActions()
		}

		if len(actions) == 0 {
			actions = []PendingAction{} // return empty array, not null
		}
		cb, err := JSONContent(actions)
		if err != nil {
			return errorResult(err.Error())
		}
		return &ToolCallResult{Content: []ContentBlock{cb}}
	})
}

// --- Process tools ---

func (s *Server) registerListProcesses() {
	s.register("list_processes", "List all running processes", json.RawMessage(`{
		"type": "object",
		"properties": {},
		"additionalProperties": false
	}`), func(args json.RawMessage) *ToolCallResult {
		if s.pm == nil {
			return errorResult("process manager not available")
		}
		ids := s.pm.IDs()
		type processInfo struct {
			ID     string `json:"id"`
			Status string `json:"status"`
		}
		result := make([]processInfo, len(ids))
		for i, id := range ids {
			result[i] = processInfo{ID: id, Status: s.pm.GetStatus(id)}
		}
		cb, err := JSONContent(result)
		if err != nil {
			return errorResult(err.Error())
		}
		return &ToolCallResult{Content: []ContentBlock{cb}}
	})
}

func (s *Server) registerCreateProcess() {
	s.register("create_process", "Create and start a background process with its own transport", json.RawMessage(`{
		"type": "object",
		"properties": {
			"process_id": {"type": "string", "description": "Unique process identifier"},
			"transport": {
				"type": "object",
				"description": "Transport configuration",
				"properties": {
					"type": {"type": "string", "enum": ["file", "llm", "interval"]},
					"path": {"type": "string"},
					"provider": {"type": "string"},
					"model": {"type": "string"},
					"prompt": {"type": "string"},
					"interval": {"type": "integer"},
					"message": {}
				},
				"required": ["type"]
			}
		},
		"required": ["process_id", "transport"],
		"additionalProperties": false
	}`), func(args json.RawMessage) *ToolCallResult {
		if s.pm == nil {
			return errorResult("process manager not available")
		}
		var p struct {
			ProcessID string          `json:"process_id"`
			Transport json.RawMessage `json:"transport"`
		}
		if err := json.Unmarshal(args, &p); err != nil {
			return errorResult("invalid params: " + err.Error())
		}
		// Build the createProcess message and route through session
		msgJSON := fmt.Sprintf(`{"type":"createProcess","processId":%s,"transport":%s}`,
			mustJSON(p.ProcessID), string(p.Transport))
		if err := s.SendMessage(json.RawMessage(msgJSON)); err != nil {
			return errorResult("create process error: " + err.Error())
		}
		return &ToolCallResult{Content: []ContentBlock{TextContent("process created: " + p.ProcessID)}}
	})
}

func (s *Server) registerStopProcess() {
	s.register("stop_process", "Stop a running background process", json.RawMessage(`{
		"type": "object",
		"properties": {
			"process_id": {"type": "string", "description": "Process ID to stop"}
		},
		"required": ["process_id"],
		"additionalProperties": false
	}`), func(args json.RawMessage) *ToolCallResult {
		if s.pm == nil {
			return errorResult("process manager not available")
		}
		var p struct {
			ProcessID string `json:"process_id"`
		}
		if err := json.Unmarshal(args, &p); err != nil {
			return errorResult("invalid params: " + err.Error())
		}
		msgJSON := fmt.Sprintf(`{"type":"stopProcess","processId":%s}`, mustJSON(p.ProcessID))
		if err := s.SendMessage(json.RawMessage(msgJSON)); err != nil {
			return errorResult("stop process error: " + err.Error())
		}
		return &ToolCallResult{Content: []ContentBlock{TextContent("process stopped: " + p.ProcessID)}}
	})
}

func (s *Server) registerSendToProcess() {
	s.register("send_to_process", "Send an A2UI message to a running process", json.RawMessage(`{
		"type": "object",
		"properties": {
			"process_id": {"type": "string", "description": "Target process ID"},
			"message": {"description": "A2UI JSONL message object to send"}
		},
		"required": ["process_id", "message"],
		"additionalProperties": false
	}`), func(args json.RawMessage) *ToolCallResult {
		if s.pm == nil {
			return errorResult("process manager not available")
		}
		var p struct {
			ProcessID string          `json:"process_id"`
			Message   json.RawMessage `json:"message"`
		}
		if err := json.Unmarshal(args, &p); err != nil {
			return errorResult("invalid params: " + err.Error())
		}
		msgJSON := fmt.Sprintf(`{"type":"sendToProcess","processId":%s,"message":%s}`,
			mustJSON(p.ProcessID), string(p.Message))
		if err := s.SendMessage(json.RawMessage(msgJSON)); err != nil {
			return errorResult("send to process error: " + err.Error())
		}
		return &ToolCallResult{Content: []ContentBlock{TextContent("message sent to process: " + p.ProcessID)}}
	})
}

// --- Channel tools ---

func (s *Server) registerListChannels() {
	s.register("list_channels", "List all channels with mode, subscribers, and last value", json.RawMessage(`{
		"type": "object",
		"properties": {},
		"additionalProperties": false
	}`), func(args json.RawMessage) *ToolCallResult {
		if s.cm == nil {
			return errorResult("channel manager not available")
		}
		ids := s.cm.IDs()
		type subInfo struct {
			ProcessID  string `json:"processId,omitempty"`
			TargetPath string `json:"targetPath,omitempty"`
		}
		type channelInfo struct {
			ID          string    `json:"id"`
			Mode        string    `json:"mode"`
			Subscribers []subInfo `json:"subscribers"`
			LastValue   any       `json:"lastValue,omitempty"`
		}
		result := make([]channelInfo, 0, len(ids))
		for _, id := range ids {
			ch := s.cm.GetChannel(id)
			if ch == nil {
				continue
			}
			subs := make([]subInfo, len(ch.Subscribers))
			for i, sub := range ch.Subscribers {
				subs[i] = subInfo{ProcessID: sub.ProcessID, TargetPath: sub.TargetPath}
			}
			result = append(result, channelInfo{
				ID:          ch.ID,
				Mode:        string(ch.Mode),
				Subscribers: subs,
				LastValue:   ch.LastValue,
			})
		}
		cb, err := JSONContent(result)
		if err != nil {
			return errorResult(err.Error())
		}
		return &ToolCallResult{Content: []ContentBlock{cb}}
	})
}

func (s *Server) registerCreateChannel() {
	s.register("create_channel", "Create a named channel for inter-process communication", json.RawMessage(`{
		"type": "object",
		"properties": {
			"channel_id": {"type": "string", "description": "Unique channel identifier"},
			"mode": {"type": "string", "enum": ["broadcast", "queue"], "description": "Delivery mode (default: broadcast)"}
		},
		"required": ["channel_id"],
		"additionalProperties": false
	}`), func(args json.RawMessage) *ToolCallResult {
		if s.cm == nil {
			return errorResult("channel manager not available")
		}
		var p struct {
			ChannelID string `json:"channel_id"`
			Mode      string `json:"mode"`
		}
		if err := json.Unmarshal(args, &p); err != nil {
			return errorResult("invalid params: " + err.Error())
		}
		mode := p.Mode
		if mode == "" {
			mode = "broadcast"
		}
		msgJSON := fmt.Sprintf(`{"type":"createChannel","channelId":%s,"mode":%s}`,
			mustJSON(p.ChannelID), mustJSON(mode))
		if err := s.SendMessage(json.RawMessage(msgJSON)); err != nil {
			return errorResult("create channel error: " + err.Error())
		}
		return &ToolCallResult{Content: []ContentBlock{TextContent("channel created: " + p.ChannelID)}}
	})
}

func (s *Server) registerDeleteChannel() {
	s.register("delete_channel", "Delete a channel and all its subscriptions", json.RawMessage(`{
		"type": "object",
		"properties": {
			"channel_id": {"type": "string", "description": "Channel ID to delete"}
		},
		"required": ["channel_id"],
		"additionalProperties": false
	}`), func(args json.RawMessage) *ToolCallResult {
		if s.cm == nil {
			return errorResult("channel manager not available")
		}
		var p struct {
			ChannelID string `json:"channel_id"`
		}
		if err := json.Unmarshal(args, &p); err != nil {
			return errorResult("invalid params: " + err.Error())
		}
		msgJSON := fmt.Sprintf(`{"type":"deleteChannel","channelId":%s}`, mustJSON(p.ChannelID))
		if err := s.SendMessage(json.RawMessage(msgJSON)); err != nil {
			return errorResult("delete channel error: " + err.Error())
		}
		return &ToolCallResult{Content: []ContentBlock{TextContent("channel deleted: " + p.ChannelID)}}
	})
}

func (s *Server) registerPublish() {
	s.register("publish", "Publish a value to a channel", json.RawMessage(`{
		"type": "object",
		"properties": {
			"channel_id": {"type": "string", "description": "Channel ID to publish to"},
			"value": {"description": "Value to publish (any JSON type)"}
		},
		"required": ["channel_id", "value"],
		"additionalProperties": false
	}`), func(args json.RawMessage) *ToolCallResult {
		if s.cm == nil {
			return errorResult("channel manager not available")
		}
		var p struct {
			ChannelID string          `json:"channel_id"`
			Value     json.RawMessage `json:"value"`
		}
		if err := json.Unmarshal(args, &p); err != nil {
			return errorResult("invalid params: " + err.Error())
		}
		msgJSON := fmt.Sprintf(`{"type":"publish","channelId":%s,"value":%s}`,
			mustJSON(p.ChannelID), string(p.Value))
		if err := s.SendMessage(json.RawMessage(msgJSON)); err != nil {
			return errorResult("publish error: " + err.Error())
		}
		return &ToolCallResult{Content: []ContentBlock{TextContent("published to channel: " + p.ChannelID)}}
	})
}

func (s *Server) registerSubscribe() {
	s.register("subscribe", "Subscribe to a channel to receive published values", json.RawMessage(`{
		"type": "object",
		"properties": {
			"channel_id": {"type": "string", "description": "Channel ID to subscribe to"},
			"process_id": {"type": "string", "description": "Process ID of the subscriber (optional)"},
			"target_path": {"type": "string", "description": "DataModel path to deliver values to (e.g. /notifications/latest)"}
		},
		"required": ["channel_id"],
		"additionalProperties": false
	}`), func(args json.RawMessage) *ToolCallResult {
		if s.cm == nil {
			return errorResult("channel manager not available")
		}
		var p struct {
			ChannelID  string `json:"channel_id"`
			ProcessID  string `json:"process_id"`
			TargetPath string `json:"target_path"`
		}
		if err := json.Unmarshal(args, &p); err != nil {
			return errorResult("invalid params: " + err.Error())
		}
		parts := fmt.Sprintf(`"channelId":%s`, mustJSON(p.ChannelID))
		if p.ProcessID != "" {
			parts += fmt.Sprintf(`,"processId":%s`, mustJSON(p.ProcessID))
		}
		if p.TargetPath != "" {
			parts += fmt.Sprintf(`,"targetPath":%s`, mustJSON(p.TargetPath))
		}
		msgJSON := fmt.Sprintf(`{"type":"subscribe",%s}`, parts)
		if err := s.SendMessage(json.RawMessage(msgJSON)); err != nil {
			return errorResult("subscribe error: " + err.Error())
		}
		return &ToolCallResult{Content: []ContentBlock{TextContent("subscribed to channel: " + p.ChannelID)}}
	})
}

func (s *Server) registerUnsubscribe() {
	s.register("unsubscribe", "Unsubscribe from a channel", json.RawMessage(`{
		"type": "object",
		"properties": {
			"channel_id": {"type": "string", "description": "Channel ID to unsubscribe from"},
			"process_id": {"type": "string", "description": "Process ID to unsubscribe (optional)"}
		},
		"required": ["channel_id"],
		"additionalProperties": false
	}`), func(args json.RawMessage) *ToolCallResult {
		if s.cm == nil {
			return errorResult("channel manager not available")
		}
		var p struct {
			ChannelID string `json:"channel_id"`
			ProcessID string `json:"process_id"`
		}
		if err := json.Unmarshal(args, &p); err != nil {
			return errorResult("invalid params: " + err.Error())
		}
		parts := fmt.Sprintf(`"channelId":%s`, mustJSON(p.ChannelID))
		if p.ProcessID != "" {
			parts += fmt.Sprintf(`,"processId":%s`, mustJSON(p.ProcessID))
		}
		msgJSON := fmt.Sprintf(`{"type":"unsubscribe",%s}`, parts)
		if err := s.SendMessage(json.RawMessage(msgJSON)); err != nil {
			return errorResult("unsubscribe error: " + err.Error())
		}
		return &ToolCallResult{Content: []ContentBlock{TextContent("unsubscribed from channel: " + p.ChannelID)}}
	})
}

func mustJSON(v any) string {
	data, _ := json.Marshal(v)
	return string(data)
}

// --- Helpers ---

func errorResult(msg string) *ToolCallResult {
	return &ToolCallResult{Content: []ContentBlock{ErrorContent(msg)}, IsError: true}
}

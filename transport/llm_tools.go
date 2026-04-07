package transport

import (
	"encoding/json"
	"fmt"
	"canopy/jlog"
	"canopy/protocol"
	"os/exec"
	"runtime"
	"sort"
	"strings"

	anyllm "github.com/mozilla-ai/any-llm-go"
)

// a2uiTools returns the A2UI tool definitions for the LLM.
func a2uiTools() []anyllm.Tool {
	return []anyllm.Tool{
		{
			Type: "function",
			Function: anyllm.Function{
				Name:        "a2ui_createSurface",
				Description: "Create a new UI surface (window). Must be called before any components can be rendered.",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"surfaceId":       map[string]any{"type": "string", "description": "Unique surface identifier"},
						"title":           map[string]any{"type": "string", "description": "Window title"},
						"width":           map[string]any{"type": "integer", "description": "Window width in points (default 800)"},
						"height":          map[string]any{"type": "integer", "description": "Window height in points (default 600)"},
						"backgroundColor": map[string]any{"type": "string", "description": "Window background color as hex (#RRGGBB)"},
						"padding":         map[string]any{"type": "integer", "description": "Root view inset in points (default 20, use -1 for edge-to-edge)"},
					},
					"required": []string{"surfaceId", "title"},
				},
			},
		},
		{
			Type: "function",
			Function: anyllm.Function{
				Name:        "a2ui_updateComponents",
				Description: "Create or update UI components on a surface. Merge semantics: call multiple times — each call adds new components or updates existing ones by componentId. Components form a tree: containers (Row, Column, Card, List) declare their children via children.static, leaf components (Text, Button, TextField, etc.) do not. Each component has a unique componentId. Props vary by type. IMPORTANT: Send at most 8-10 components per call. Group by logical section and call multiple times.",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"surfaceId": map[string]any{"type": "string"},
						"components": map[string]any{
							"type": "array",
							"items": map[string]any{
								"type": "object",
								"properties": map[string]any{
									"componentId": map[string]any{"type": "string"},
									"type":        map[string]any{"type": "string", "enum": []string{"Text", "Row", "Column", "Card", "Button", "TextField", "CheckBox", "Slider", "Image", "Icon", "Divider", "List", "Tabs", "Modal", "ChoicePicker", "DateTimeInput", "SplitView", "OutlineView", "RichTextEditor", "SearchField"}},
									"children":    map[string]any{"type": "object", "description": "Tree structure: {\"static\": [\"childId1\", \"childId2\"]}. Required on containers."},
									"props":       map[string]any{"type": "object"},
									"style": map[string]any{
										"type":        "object",
										"description": "Visual styling overrides. All values accept literals OR dynamic values ({\"path\":\"/x\"} or {\"functionCall\":{...}}).",
										"properties": map[string]any{
											"backgroundColor": map[string]any{"description": "Background color (#RRGGBB) or dynamic value"},
											"textColor":       map[string]any{"description": "Text color (#RRGGBB) or dynamic value"},
											"cornerRadius":    map[string]any{"description": "Corner radius in points (number or dynamic value)"},
											"width":           map[string]any{"description": "Fixed width in points (number or dynamic value)"},
											"height":          map[string]any{"description": "Fixed height in points (number or dynamic value)"},
											"fontSize":        map[string]any{"description": "Font size in points (number or dynamic value)"},
											"fontWeight":      map[string]any{"description": "Font weight: \"bold\"|\"semibold\"|\"medium\"|\"light\" (or dynamic value)"},
											"fontFamily":      map[string]any{"description": "Font family name (system or registered via a2ui_loadAssets, or dynamic value)"},
											"textAlign":       map[string]any{"description": "Text alignment: \"left\"|\"center\"|\"right\" (or dynamic value)"},
											"opacity":         map[string]any{"description": "Opacity 0.0-1.0 (number or dynamic value)"},
											"flexGrow":        map[string]any{"description": "Flex grow factor (number or dynamic value). Expands to fill available space in parent stack."},
										},
									},
								},
								"required": []string{"componentId", "type"},
							},
						},
					},
					"required": []string{"surfaceId", "components"},
				},
			},
		},
		{
			Type: "function",
			Function: anyllm.Function{
				Name:        "a2ui_updateDataModel",
				Description: "Apply JSON Patch operations to a surface's data model. Use this to set initial values, update state, or remove data. Components bound to affected paths automatically re-render.",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"surfaceId": map[string]any{"type": "string"},
						"ops": map[string]any{
							"type": "array",
							"items": map[string]any{
								"type": "object",
								"properties": map[string]any{
									"op":    map[string]any{"type": "string", "enum": []string{"add", "replace", "remove"}},
									"path":  map[string]any{"type": "string", "description": "JSON Pointer (e.g. /users/0/name)"},
									"value": map[string]any{},
								},
								"required": []string{"op", "path"},
							},
						},
					},
					"required": []string{"surfaceId", "ops"},
				},
			},
		},
		// a2ui_deleteSurface intentionally omitted from LLM tools — destroying
		// and recreating surfaces wastes turns and can crash. Fix layout instead.
		{
			Type: "function",
			Function: anyllm.Function{
				Name:        "a2ui_setTheme",
				Description: "Change the visual theme of a surface.",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"surfaceId": map[string]any{"type": "string"},
						"theme":     map[string]any{"type": "string", "enum": []string{"light", "dark", "system"}},
					},
					"required": []string{"surfaceId", "theme"},
				},
			},
		},
		{
			Type: "function",
			Function: anyllm.Function{
				Name:        "a2ui_updateMenu",
				Description: "Set the native menu bar for a surface's window. Menu items can use standardAction (AppKit selector routed through responder chain, e.g. \"copy:\" for Cmd+C) or action (custom callback, same as button onClick). This is required for keyboard shortcuts like Cmd+C/V/X/Z to work in text editors.",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"surfaceId": map[string]any{"type": "string"},
						"items": map[string]any{
							"type":        "array",
							"description": "Top-level menu items (each becomes a menu bar entry with children as dropdown items)",
							"items": map[string]any{
								"type": "object",
								"properties": map[string]any{
									"id":             map[string]any{"type": "string", "description": "Unique menu item identifier"},
									"label":          map[string]any{"type": "string", "description": "Display text"},
									"keyEquivalent":  map[string]any{"type": "string", "description": "Keyboard shortcut letter (e.g. \"c\" for Cmd+C, \"Z\" uppercase for Cmd+Shift+Z)"},
									"keyModifiers":   map[string]any{"type": "string", "description": "Modifier override: \"option\" for Opt+key, \"shift\" for Shift+key. Default is Cmd."},
									"separator":      map[string]any{"type": "boolean", "description": "True for a separator line"},
									"standardAction": map[string]any{"type": "string", "description": "AppKit selector string (e.g. \"copy:\", \"paste:\", \"undo:\", \"selectAll:\"). Routes through responder chain."},
									"action":         map[string]any{"type": "object", "description": "Custom action — takes the SAME value as button onClick, i.e. {\"action\":{\"functionCall\":{\"call\":\"updateDataModel\",\"args\":{\"ops\":[...]}}}}. Note the double 'action' nesting."},
									"children": map[string]any{
										"type":  "array",
										"items": map[string]any{"type": "object"},
									},
								},
							},
						},
					},
					"required": []string{"surfaceId", "items"},
				},
			},
		},
		{
			Type: "function",
			Function: anyllm.Function{
				Name:        "a2ui_test",
				Description: "Define a test case with assertions and simulations to verify the UI. Tests run headlessly and validate component state, data model values, child relationships, actions, and layout.",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"surfaceId": map[string]any{"type": "string"},
						"name":      map[string]any{"type": "string", "description": "Test case name"},
						"steps": map[string]any{
							"type": "array",
							"items": map[string]any{
								"type": "object",
								"properties": map[string]any{
									"assert":        map[string]any{"type": "string", "enum": []string{"component", "dataModel", "children", "notExists", "count", "action", "layout", "style"}},
									"simulate":      map[string]any{"type": "string", "enum": []string{"event"}},
									"componentId":   map[string]any{"type": "string"},
									"componentType": map[string]any{"type": "string"},
									"props":         map[string]any{"type": "object"},
									"path":          map[string]any{"type": "string"},
									"value":         map[string]any{},
									"children":      map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
									"count":         map[string]any{"type": "integer"},
									"name":          map[string]any{"type": "string"},
									"data":          map[string]any{"type": "object"},
									"layout":        map[string]any{"type": "object"},
								"style":         map[string]any{"type": "object"},
									"event":         map[string]any{"type": "string"},
									"eventData":     map[string]any{"type": "string"},
								},
							},
						},
					},
					"required": []string{"surfaceId", "name", "steps"},
				},
			},
		},
		{
			Type: "function",
			Function: anyllm.Function{
				Name:        "a2ui_loadAssets",
				Description: "Declare fonts, images, audio, or video assets by alias. Fonts are registered with the system (process scope) so they become available via fontFamily in style. Images are preloaded and cached. Components can reference any asset via the \"asset:<alias>\" prefix in string props like src.",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"assets": map[string]any{
							"type": "array",
							"items": map[string]any{
								"type": "object",
								"properties": map[string]any{
									"alias": map[string]any{"type": "string", "description": "Short name to reference this asset (e.g. \"BrandFont\", \"hero\")"},
									"kind":  map[string]any{"type": "string", "enum": []string{"font", "image", "audio", "video"}, "description": "Asset type"},
									"src":   map[string]any{"type": "string", "description": "Local path (absolute or relative to CWD) or URL"},
								},
								"required": []string{"alias", "kind", "src"},
							},
						},
					},
					"required": []string{"assets"},
				},
			},
		},
		{
			Type: "function",
			Function: anyllm.Function{
				Name:        "a2ui_loadLibrary",
				Description: "Load a native dynamic library (.dylib/.so/.dll) and register its functions for use in component expressions via functionCall. Functions are called with their actual C signatures via libffi — no wrappers needed. Declare each function's return type and parameter types.",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"path":   map[string]any{"type": "string", "description": "Absolute path to the dynamic library file"},
						"prefix": map[string]any{"type": "string", "description": "Namespace prefix for registered functions (e.g. 'curl' → callable as curl.init)"},
						"functions": map[string]any{
							"type": "array",
							"items": map[string]any{
								"type": "object",
								"properties": map[string]any{
									"name":       map[string]any{"type": "string", "description": "Function name used in expressions (e.g. 'init')"},
									"symbol":     map[string]any{"type": "string", "description": "Exported C symbol name in the library (e.g. 'curl_easy_init')"},
									"returnType": map[string]any{"type": "string", "enum": []string{"void", "int", "uint32", "int64", "uint64", "float", "double", "pointer", "string", "bool"}, "description": "C return type"},
									"paramTypes": map[string]any{"type": "array", "items": map[string]any{"type": "string", "enum": []string{"int", "uint32", "int64", "uint64", "float", "double", "pointer", "string", "bool"}}, "description": "C parameter types in order"},
									"fixedArgs":  map[string]any{"type": "integer", "description": "For variadic functions: number of fixed args before the variadic part"},
								},
								"required": []string{"name", "symbol", "returnType", "paramTypes"},
							},
						},
					},
					"required": []string{"path", "prefix", "functions"},
				},
			},
		},
		{
			Type: "function",
			Function: anyllm.Function{
				Name:        "a2ui_inspectLibrary",
				Description: "Inspect a native dynamic library to discover its exported symbols. Returns a list of exported function symbols that can be used with a2ui_loadLibrary. Any exported C function can be called — declare its return type and parameter types when loading.",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"path": map[string]any{"type": "string", "description": "Absolute path to the dynamic library file (.dylib/.so/.dll)"},
					},
					"required": []string{"path"},
				},
			},
		},
		{
			Type: "function",
			Function: anyllm.Function{
				Name:        "a2ui_defineFunction",
				Description: "Define a reusable function with parameters. AVOID for complex logic — use inline function composition and data model lookup tables instead (see system prompt). Only use for trivially simple helpers (1-2 nested levels). The body is a JSON expression tree where {\"param\":\"name\"} nodes get replaced with argument values at call time.",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"name":   map[string]any{"type": "string", "description": "Function name (must not conflict with built-in functions)"},
						"params": map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Parameter names (positional)"},
						"body":   map[string]any{"type": "object", "description": "JSON expression tree with {\"param\":\"name\"} placeholders. MUST be an object like {\"functionCall\":{...}} or {\"param\":\"x\"} — NEVER a JSON-encoded string. If you accidentally send a string it will be auto-parsed but this is an error."},
					},
					"required": []string{"name", "params", "body"},
				},
			},
		},
		{
			Type: "function",
			Function: anyllm.Function{
				Name:        "a2ui_defineComponent",
				Description: "Define a reusable component template with parameters. Use {\"param\":\"name\"} in props/style/actions for parameterization. Use \"$/path\" for scoped data paths (replaced with scope prefix at instantiation). The definition must have exactly one component with componentId \"_root\". Instantiate with useComponent in a2ui_updateComponents.",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"name":   map[string]any{"type": "string", "description": "Component template name"},
						"params": map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Parameter names"},
						"components": map[string]any{
							"type":        "array",
							"description": "Component definitions forming the template. Use _root as the main component's ID.",
							"items":       map[string]any{"type": "object"},
						},
					},
					"required": []string{"name", "params", "components"},
				},
			},
		},
		{
			Type: "function",
			Function: anyllm.Function{
				Name:        "a2ui_createProcess",
				Description: "Start a managed background process with its own transport. Processes can read JSONL files, connect to LLMs, or send messages on a timer. Process status is written to /processes/{processId}/status in the data model of all surfaces.",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"processId": map[string]any{"type": "string", "description": "Unique process identifier"},
						"transport": map[string]any{
							"type":        "object",
							"description": "Transport configuration",
							"properties": map[string]any{
								"type":     map[string]any{"type": "string", "enum": []string{"file", "llm", "interval"}, "description": "Transport type"},
								"path":     map[string]any{"type": "string", "description": "File transport: JSONL file path"},
								"provider": map[string]any{"type": "string", "description": "LLM transport: provider name"},
								"model":    map[string]any{"type": "string", "description": "LLM transport: model name"},
								"prompt":   map[string]any{"type": "string", "description": "LLM transport: system prompt"},
								"interval": map[string]any{"type": "integer", "description": "Interval transport: milliseconds between messages"},
								"message":  map[string]any{"description": "Interval transport: A2UI JSONL message to send on each tick"},
							},
							"required": []string{"type"},
						},
					},
					"required": []string{"processId", "transport"},
				},
			},
		},
		{
			Type: "function",
			Function: anyllm.Function{
				Name:        "a2ui_stopProcess",
				Description: "Stop a running background process.",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"processId": map[string]any{"type": "string", "description": "Process ID to stop"},
					},
					"required": []string{"processId"},
				},
			},
		},
		{
			Type: "function",
			Function: anyllm.Function{
				Name:        "a2ui_createChannel",
				Description: "Create a named channel for inter-process communication. Published values are written to /channels/{channelId}/value in the data model. Broadcast mode delivers to all subscribers; queue mode delivers round-robin to one subscriber at a time.",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"channelId": map[string]any{"type": "string", "description": "Unique channel identifier"},
						"mode":      map[string]any{"type": "string", "enum": []string{"broadcast", "queue"}, "description": "Delivery mode (default: broadcast)"},
					},
					"required": []string{"channelId"},
				},
			},
		},
		{
			Type: "function",
			Function: anyllm.Function{
				Name:        "a2ui_deleteChannel",
				Description: "Delete a channel and all its subscriptions.",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"channelId": map[string]any{"type": "string", "description": "Channel ID to delete"},
					},
					"required": []string{"channelId"},
				},
			},
		},
		{
			Type: "function",
			Function: anyllm.Function{
				Name:        "a2ui_publish",
				Description: "Publish a value to a channel. The value is written to /channels/{channelId}/value and delivered to all subscribers (broadcast) or one subscriber (queue).",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"channelId": map[string]any{"type": "string", "description": "Channel ID to publish to"},
						"value":     map[string]any{"description": "Value to publish (any JSON type)"},
					},
					"required": []string{"channelId", "value"},
				},
			},
		},
		{
			Type: "function",
			Function: anyllm.Function{
				Name:        "a2ui_subscribe",
				Description: "Subscribe to a channel. When a value is published, it is written to the targetPath in the data model of all surfaces. Components bound to that path auto-update.",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"channelId":  map[string]any{"type": "string", "description": "Channel ID to subscribe to"},
						"processId":  map[string]any{"type": "string", "description": "Process ID of the subscriber (optional, omit for session-level)"},
						"targetPath": map[string]any{"type": "string", "description": "DataModel path to deliver values to (e.g. /notifications/latest)"},
					},
					"required": []string{"channelId"},
				},
			},
		},
		{
			Type: "function",
			Function: anyllm.Function{
				Name:        "a2ui_unsubscribe",
				Description: "Unsubscribe from a channel.",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"channelId": map[string]any{"type": "string", "description": "Channel ID to unsubscribe from"},
						"processId": map[string]any{"type": "string", "description": "Process ID to unsubscribe (optional)"},
					},
					"required": []string{"channelId"},
				},
			},
		},
		{
			Type: "function",
			Function: anyllm.Function{
				Name:        "a2ui_updateToolbar",
				Description: "Set the native toolbar for a surface's window. Toolbar items appear as buttons in the window titlebar. Items can have icons (SF Symbols), labels, custom actions or standard AppKit selectors. Use bordered:true for rounded button appearance, selected for toggle state.",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"surfaceId": map[string]any{"type": "string"},
						"items": map[string]any{
							"type":        "array",
							"description": "Toolbar items in order. Use separator:true or flexible:true for spacing.",
							"items": map[string]any{
								"type": "object",
								"properties": map[string]any{
									"id":             map[string]any{"type": "string", "description": "Unique toolbar item identifier"},
									"icon":           map[string]any{"type": "string", "description": "SF Symbol name (e.g. \"bold\", \"italic\", \"sidebar.left\")"},
									"label":          map[string]any{"type": "string", "description": "Tooltip / text label"},
									"standardAction": map[string]any{"type": "string", "description": "AppKit selector (e.g. \"toggleBoldface:\", \"toggleSidebar:\"). Routes through responder chain."},
									"action":         map[string]any{"type": "object", "description": "Custom action — takes the SAME value as button onClick, i.e. {\"action\":{\"functionCall\":{\"call\":\"updateDataModel\",\"args\":{\"ops\":[...]}}}}. Note the double 'action' nesting."},
									"separator":      map[string]any{"type": "boolean", "description": "Thin divider between items"},
									"flexible":       map[string]any{"type": "boolean", "description": "Flexible space (pushes items apart)"},
									"searchField":    map[string]any{"type": "boolean", "description": "Render as NSSearchToolbarItem instead of button"},
									"dataBinding":    map[string]any{"type": "string", "description": "JSON pointer for search field text binding"},
									"bordered":       map[string]any{"type": "boolean", "description": "Rounded button appearance (macOS 11+)"},
									"selected":       map[string]any{"description": "Toggle/highlight state (bool or {\"path\":\"/x\"} for dynamic binding)"},
								},
							},
						},
					},
					"required": []string{"surfaceId", "items"},
				},
			},
		},
		{
			Type: "function",
			Function: anyllm.Function{
				Name:        "a2ui_updateWindow",
				Description: "Update window properties after creation. Set title, minimum size constraints.",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"surfaceId": map[string]any{"type": "string"},
						"title":     map[string]any{"type": "string", "description": "New window title"},
						"minWidth":  map[string]any{"type": "integer", "description": "Minimum window width in points"},
						"minHeight": map[string]any{"type": "integer", "description": "Minimum window height in points"},
					},
					"required": []string{"surfaceId"},
				},
			},
		},
		{
			Type: "function",
			Function: anyllm.Function{
				Name:        "a2ui_takeScreenshot",
				Description: "Take a screenshot of the current window. Returns the visual state as an image so you can verify layout, spacing, and component rendering. Use this after building your UI to check the result visually.",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"surfaceId": map[string]any{"type": "string", "description": "Surface to screenshot"},
					},
					"required": []string{"surfaceId"},
				},
			},
		},
		{
			Type: "function",
			Function: anyllm.Function{
				Name:        "a2ui_getLogs",
				Description: "Query application logs. Use this to inspect errors, debug binding issues, and understand what happened in the app. Returns matching log entries with level, component, surface, and message.",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"level":      map[string]any{"type": "string", "description": "Minimum level: debug, info, warn, error (default info)"},
						"component":  map[string]any{"type": "string", "description": "Filter by component (session, transport, darwin, mcp, resolver, etc.)"},
						"surface_id": map[string]any{"type": "string", "description": "Filter by surface ID"},
						"pattern":    map[string]any{"type": "string", "description": "Regex pattern to match message text"},
						"limit":      map[string]any{"type": "integer", "description": "Max entries (default 50, max 500)"},
						"offset":     map[string]any{"type": "integer", "description": "Skip first N matches (default 0)"},
					},
				},
			},
		},
	}
}

// toolCallToMessage converts a tool call into a protocol.Message by injecting the "type" field
// and parsing through the standard protocol parser path. Returns the parsed message and the
// raw JSONL bytes (for recording).
func toolCallToMessage(tc anyllm.ToolCall) (*protocol.Message, []byte, error) {
	name := tc.Function.Name
	if !strings.HasPrefix(name, "a2ui_") {
		return nil, nil, fmt.Errorf("unknown tool call: %s", name)
	}
	msgType := name[5:] // strip "a2ui_" prefix

	// Parse the arguments JSON
	var args map[string]json.RawMessage
	if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
		return nil, nil, fmt.Errorf("parse tool call args: %w", err)
	}

	// Auto-unwrap string-encoded fields. LLMs sometimes serialize arrays/objects
	// as JSON strings (e.g. "components": "[{...}]" instead of "components": [{...}]).
	// Detect and unwrap these to prevent parse errors.
	for key, raw := range args {
		if len(raw) > 2 && raw[0] == '"' {
			var str string
			if err := json.Unmarshal(raw, &str); err == nil {
				trimmed := strings.TrimSpace(str)
				if len(trimmed) > 0 && (trimmed[0] == '[' || trimmed[0] == '{') {
					var parsed json.RawMessage
					if err := json.Unmarshal([]byte(trimmed), &parsed); err == nil {
						args[key] = parsed
						jlog.Warnf("transport", "", "tool call %s: auto-unwrapped string-encoded field %q", name, key)
					}
				}
			}
		}
	}

	// Inject the "type" field
	typeBytes, _ := json.Marshal(msgType)
	args["type"] = typeBytes

	// Re-serialize to a complete JSONL line
	line, err := json.Marshal(args)
	if err != nil {
		return nil, nil, fmt.Errorf("serialize tool call: %w", err)
	}

	// Parse through the standard protocol parser
	parser := protocol.NewParser(strings.NewReader(string(line)))
	msg, err := parser.Next()
	if err != nil {
		return nil, nil, err
	}
	return msg, line, nil
}

// categoryOrder defines the display order and heading for each function category.
var categoryOrder = []struct {
	key   string
	label string
}{
	{"string", "String functions"},
	{"math", "Math functions"},
	{"logic", "Logic functions"},
	{"array", "Array/list functions"},
	{"object", "Object functions"},
	{"system", "System functions"},
}

// functionDocsForPrompt generates the AVAILABLE FUNCTIONS block from the registry.
func functionDocsForPrompt() string {
	// Group functions by category
	byCategory := make(map[string][]protocol.FuncMeta)
	for _, f := range protocol.FunctionRegistry {
		byCategory[f.Category] = append(byCategory[f.Category], f)
	}

	var b strings.Builder
	b.WriteString("AVAILABLE FUNCTIONS:\n")
	for i, cat := range categoryOrder {
		funcs := byCategory[cat.key]
		if len(funcs) == 0 {
			continue
		}
		b.WriteString(cat.label)
		b.WriteString(":\n")
		for _, f := range funcs {
			fmt.Fprintf(&b, "- %s(%s) — %s\n", f.Name, f.Args, f.Desc)
		}
		if i < len(categoryOrder)-1 {
			b.WriteString("\n")
		}
	}

	// Warn about any unknown categories
	known := make(map[string]bool)
	for _, cat := range categoryOrder {
		known[cat.key] = true
	}
	var unknown []string
	for cat := range byCategory {
		if !known[cat] {
			unknown = append(unknown, cat)
		}
	}
	if len(unknown) > 0 {
		sort.Strings(unknown)
		for _, cat := range unknown {
			b.WriteString("\n")
			b.WriteString(cat)
			b.WriteString(" functions:\n")
			for _, f := range byCategory[cat] {
				fmt.Fprintf(&b, "- %s(%s) — %s\n", f.Name, f.Args, f.Desc)
			}
		}
	}

	return b.String()
}

// ComponentReference returns the A2UI protocol reference text (the system prompt
// without a user request appended). Used as MCP resource for Claude Code transport.
// If libraryBlock is non-empty, it is appended to the reference text.
func ComponentReference(libraryBlock ...string) string {
	ref := SystemPrompt("")
	if len(libraryBlock) > 0 && libraryBlock[0] != "" {
		ref += "\n" + libraryBlock[0]
	}
	return ref
}

// SystemPrompt returns the system message that teaches the LLM about A2UI.
func SystemPrompt(userPrompt string) string {
	return `You are a UI builder. You create native macOS user interfaces using the A2UI protocol via tool calls.

AVAILABLE COMPONENTS:
- Text: Display text. Props: content (string), variant ("h1"|"h2"|"h3"|"body"|"caption")
- Row: Horizontal layout. Props: gap (int), padding (int), justify ("start"|"center"|"end"|"spaceBetween"|"spaceAround"|"fillEqually"), align ("start"|"center"|"end"|"stretch")
- Column: Vertical layout. Props: gap (int), padding (int), justify, align (same values as Row)
- Card: Titled container. Props: title (string), subtitle (string)
- Button: Clickable button. Props: label (string), style ("primary"|"secondary"|"destructive"), onClick (see CLIENT-SIDE ACTIONS below)
- TextField: Text input. Props: placeholder (string), value (string), dataBinding (JSON pointer)
- CheckBox: Toggle. Props: label (string), checked (bool), dataBinding (JSON pointer)
- Slider: Range input. Props: min (number), max (number), step (number), sliderValue (number), dataBinding (JSON pointer)
- Image: Display image. Props: src (URL string or "asset:<alias>"), alt (string), width (int), height (int)
- Icon: SF Symbol. Props: name (string), size (int)
- Divider: Visual separator. No props needed.
- List: Scrollable list container.
- Tabs: Tabbed container showing one child at a time. Props: tabLabels (array of strings, one per child), activeTab (child ID of selected tab), dataBinding (JSON pointer to store selected child ID). Children define tab content.
- Modal: Floating panel window. Props: title (string), visible (bool or data binding), dataBinding (JSON pointer for two-way visible binding), width (int), height (int), onDismiss (action). Children define modal content. When dismissed, dataBinding is set to false.
- ChoicePicker: Dropdown/selection. Props: options (array of {value, label}), dataBinding (JSON pointer), mutuallyExclusive (bool)
- DateTimeInput: Date/time picker. Props: enableDate (bool), enableTime (bool), dataBinding (JSON pointer)
- SplitView: Multi-pane resizable layout (NSSplitView). Props: dividerStyle ("thin"|"thick"|"paneSplitter"), vertical (bool, REQUIRED — true for side-by-side panes like a 3-column layout), collapsedPane (number or functionCall, pane index to collapse, -1=none). Children = panes. IMPORTANT: Set style.width on the CHILD pane components (e.g. sidebar Column gets width:200), NOT on the SplitView itself. The SplitView should have NO width — it fills its parent. Always set vertical:true for multi-column layouts.
- OutlineView: Hierarchical tree view (NSOutlineView). Props: outlineData (path to tree array), labelKey (string, default "name"), childrenKey (string, default "children"), iconKey (string, SF Symbol key), idKey (string, default "id"), selectedId (string or path), badgeKey (string, numeric badge hidden when 0), dataBinding (JSON pointer for selection)
- RichTextEditor: Rich text editor (NSTextView). Props: richContent (markdown string or path), editable (bool, default true), formatBinding (JSON pointer for cursor format state: bold/italic/underline/strikethrough booleans). NEVER use dataBinding on RichTextEditor — it conflicts with richContent+onChange and causes dual-write corruption. CRITICAL: You MUST add an onChange handler to persist edits — without it, all typing is lost. onChange receives the new content at /_input path. Example: "onChange":{"action":{"functionCall":{"call":"updateDataModel","args":{"ops":[{"op":"replace","path":"/notes","value":{"functionCall":{"name":"updateItem","args":[{"path":"/notes"},"id",{"path":"/selectedId"},"content",{"path":"/_input"}]}}}]}}}}
- SearchField: Native search input (NSSearchField). Props: placeholder (string), value (string or path), dataBinding (JSON pointer for search text)

CLIENT-SIDE ACTIONS (Button onClick):
Buttons can trigger client-side data model updates via functionCall. The onClick prop uses this EXACT structure:

"onClick": {
  "action": {
    "functionCall": {
      "call": "updateDataModel",
      "args": {
        "ops": [
          {"op": "replace", "path": "/someKey", "value": "newValue"}
        ]
      }
    }
  }
}

The only supported call is "updateDataModel". Each op has:
- "op": "replace" | "add" | "remove"
- "path": JSON Pointer string (e.g. "/display", "/result")
- "value": the new value (can be a literal, a path reference, or a functionCall — see DYNAMIC VALUES)

For standard AppKit actions (routed through responder chain, e.g. formatting in text editors):
"onClick": {
  "action": {
    "standardAction": "toggleBoldface:"
  }
}
Common selectors: toggleBoldface:, toggleItalics:, underline:, undo:, redo:, copy:, paste:, cut:, selectAll:

TOOLBAR, MENU, AND CONTEXT MENU ACTIONS:
The "action" field on toolbar items, menu items, and context menu items takes the SAME value as button onClick.
Since onClick = {"action": {"functionCall": ...}}, the toolbar/menu/context menu action field becomes:

"action": {
  "action": {
    "functionCall": {
      "call": "updateDataModel",
      "args": {"ops": [...]}
    }
  }
}

YES, there are two nested "action" keys — the outer one is the field name, the inner one is part of the EventAction structure (same as onClick).
WRONG (will silently fail): "action": {"functionCall": {"call": "updateDataModel", ...}}
RIGHT: "action": {"action": {"functionCall": {"call": "updateDataModel", ...}}}

For standard AppKit actions on toolbar/menu items, use the top-level standardAction field instead:
{"id": "bold", "standardAction": "toggleBoldface:", ...}
Do NOT put standardAction inside the "action" field.

For server-side actions (sending events back to the LLM), use this structure instead:
"onClick": {
  "action": {
    "event": {
      "name": "myAction",
      "dataRefs": ["/path1", "/path2"]
    }
  }
}

DYNAMIC VALUES:
Anywhere a "value" appears in an updateDataModel op, it can be one of:

1. A literal: "hello", 42, true, false
2. A path reference (reads current value from data model): {"path": "/display"}
3. A function call: {"functionCall": {"name": "concat", "args": ["hello", " ", "world"]}}

Function call args are POSITIONAL (an array), and each arg can itself be a literal, path ref, or nested function call.

IMPORTANT: Do NOT invent syntax like {"$fn": ...}, {"$ref": ...}, or named parameters like {"condition": ..., "then": ..., "else": ...}. The ONLY valid object forms in a value are {"path": "..."} and {"functionCall": {"name": "...", "args": [...]}}.

IMPORTANT: Only use functions listed in AVAILABLE FUNCTIONS below. Do NOT invent functions.

SHELL EXECUTION:
Use shell(command) to run external commands and capture output. The command arg can be a literal or a functionCall (e.g. concat).
Example — whois lookup: shell(concat("whois ", {path: "/domain"})) runs "whois example.com" and returns the output as a string.
This makes apps self-contained: buttons can run curl, whois, dig, etc. without server-side polling.

Common patterns:
- Count matching items: countWhere(list, key, value) or length(filter(list, key, value))
- Truncate string: substring(str, 0, 80) — substring safely handles strings shorter than the limit, no min/if needed
- Get nested field from found item: getField(find(list, idKey, idValue), fieldName)
- Look up display name from map: getField(/folderNames, selectedId) — store maps in data model, not in defineFunction if-chains
- Multi-field search: filterContainsAny(list, ["title","content"], query) — case-insensitive search across multiple string fields

CRITICAL PATTERNS (common mistakes that break apps):

1. ID GENERATION: Use a monotonic counter in the data model (e.g. /nextNoteNum), NOT length(array)+1. The length-based approach breaks because: (a) operations in a single updateDataModel execute sequentially, so length changes after an append — the second op computes a DIFFERENT ID; (b) deletions cause ID collisions. Correct pattern:
   ops: [
     {op:"replace", path:"/selectedId", value: concat("n", toString(/nextNoteNum))},  ← select FIRST
     {op:"replace", path:"/items", value: append(/items, {id: concat("n", toString(/nextNoteNum)), ...})},  ← append SECOND (counter unchanged)
     {op:"replace", path:"/nextNoteNum", value: add(/nextNoteNum, 1)}  ← increment LAST
   ]

2. SEARCH MUST BE GLOBAL: When implementing search + folder filtering, the search branch must filter the ENTIRE collection, NOT a folder-filtered subset. Wrong: filterContainsAny(filter(notes, folderId, x), ...). Right: if(searchActive, filterContainsAny(/notes, fields, query), folderFilter). Search and folder filter are alternative branches, never nested.

3. OUTLINEVIEW TREE NAVIGATION: Every node in the tree (parents AND leaves) can be selected. Parent nodes like "iCloud" or "On My Mac" don't correspond to any folderId. You MUST handle all clickable node IDs in filter logic. Best practice: use the same string for tree node IDs and the account/folderId values in your data. For parent-level nodes, switch to account-based filtering (e.g., clicking "iCloud" → filter by account="icloud"). NEVER assume only leaf nodes will be selected.

` + functionDocsForPrompt() + `

EXAMPLE — Calculator digit button using dynamic values:
This button appends "7" to display, or replaces "0" with "7":
{
  "componentId": "btn7", "type": "Button",
  "props": {
    "label": "7",
    "onClick": {
      "action": {
        "functionCall": {
          "call": "updateDataModel",
          "args": {
            "ops": [{
              "op": "replace",
              "path": "/display",
              "value": {
                "functionCall": {
                  "name": "if",
                  "args": [
                    {"functionCall": {"name": "equals", "args": [{"path": "/display"}, "0"]}},
                    "7",
                    {"functionCall": {"name": "concat", "args": [{"path": "/display"}, "7"]}}
                  ]
                }
              }
            }]
          }
        }
      }
    }
  }
}

VISUAL STYLING:
Any component can have a "style" object alongside "props" with these properties:
- backgroundColor: hex color (#RRGGBB) — sets background; on Buttons, switches to borderless mode
- textColor: hex color — sets text/tint color
- cornerRadius: number — rounds corners
- width/height: number — fixed dimensions in points
- fontSize: number — font size in points
- fontWeight: "bold"|"semibold"|"medium"|"light"
- fontFamily: font family name (e.g. "Comic Sans MS", "Courier New", or a custom font registered via a2ui_loadAssets)
- textAlign: "left"|"center"|"right"
- opacity: 0.0-1.0
- flexGrow: number — expand to fill available space in parent stack (e.g. 1 fills remaining space)

ALL style properties accept dynamic values: {"path": "/someColor"} or {"functionCall": {"name": "if", "args": [...]}}
Example dynamic backgroundColor: "style": {"backgroundColor": {"functionCall": {"name": "if", "args": [{"path": "/selected"}, "#007AFF", "#FFFFFF"]}}}

Text component also supports: maxLines (int, 0=unlimited, 1+=truncate with ellipsis)

Surface-level styling:
- backgroundColor on createSurface sets the window background color
- padding on createSurface sets root inset (default 20, use -1 for edge-to-edge)

Layout tip: use justify:"fillEqually" on Row/Column to make all children equal-width/height.

DYNAMIC CHILDREN (forEach):
Instead of static children, containers can iterate over a data model array:
"children": {"forEach": "/items", "templateId": "item_tmpl", "itemVariable": "item"}
- forEach: JSON pointer to an array in the data model, OR a functionCall that returns an array (for filtering/sorting)
- templateId: componentId of the template (defined alongside, not rendered directly)
- itemVariable: variable name; template components use "/item/field" paths to access item data
Template components are cloned per array element with rewritten paths. Adding/removing items from the data model array automatically adds/removes rendered components.

IMPORTANT: To filter or sort the list, put the logic INSIDE the forEach value as a functionCall:
"children": {"forEach": {"functionCall": {"name": "sort", "args": [{"functionCall": {"name": "filter", "args": [{"path": "/notes"}, "folderId", {"path": "/selectedFolderId"}]}}, "modified", true]}}, "templateId": "note_row", "itemVariable": "note"}
Do NOT add a separate "filter" key — it is not supported and will be ignored.

CONTEXT MENUS:
Any component can have a "contextMenu" prop (in props) for right-click menus:
"contextMenu": [
  {"id": "edit", "label": "Edit", "icon": "pencil", "action": {"action": {"functionCall": {"call": "updateDataModel", "args": {"ops": [...]}}}}},
  {"separator": true},
  {"id": "delete", "label": "Delete", "icon": "trash", "action": {"action": {"functionCall": {"call": "updateDataModel", "args": {"ops": [...]}}}}}
]
Note: action uses the SAME double-nested structure as toolbar/menu actions (see TOOLBAR, MENU, AND CONTEXT MENU ACTIONS above).

NATIVE LIBRARIES (FFI):
You can load ANY native dynamic library at runtime and call its functions directly with their real C signatures — no wrappers needed.

1. Use a2ui_inspectLibrary to discover exported symbols in a .dylib/.so/.dll file
2. Use a2ui_loadLibrary to load the library, declaring each function's return type and parameter types
3. Use the registered functions in component props via functionCall: {"functionCall": {"name": "prefix.funcName", "args": [...]}}

Supported types: void, int, uint32, int64, uint64, float, double, pointer, string, bool

Type mapping:
- Numbers in JSON map to the declared C type (int→int32, double→double, etc.)
- Strings in JSON map to const char*
- Booleans in JSON map to int (0/1)
- Pointer returns give you a handle ID (a number). Pass that handle ID back to functions expecting a pointer arg.
- For variadic C functions (like printf), set fixedArgs to the number of fixed parameters before the ... part.

Example — calling libcurl:
{"type":"loadLibrary","path":"/usr/lib/libcurl.dylib","prefix":"curl","functions":[
  {"name":"init","symbol":"curl_easy_init","returnType":"pointer","paramTypes":[]},
  {"name":"perform","symbol":"curl_easy_perform","returnType":"int","paramTypes":["pointer"]},
  {"name":"cleanup","symbol":"curl_easy_cleanup","returnType":"void","paramTypes":["pointer"]}
]}

ASSETS:
Use a2ui_loadAssets to register fonts and preload images BEFORE creating components that use them.
- Font assets: register a .ttf/.otf file by alias. The font becomes available via fontFamily in style (use the font's family name, e.g. "Comic Sans MS").
  System fonts in /System/Library/Fonts/ and /System/Library/Fonts/Supplemental/ are always available without loading.
- Image assets: preload by alias. Reference in Image src with "asset:<alias>" prefix (e.g. "asset:hero").
  Images can also use direct URLs without loadAssets — the asset system is for preloading and aliasing.

Example:
a2ui_loadAssets({"assets": [
  {"alias": "logo", "kind": "image", "src": "https://example.com/logo.png"},
  {"alias": "CustomFont", "kind": "font", "src": "/path/to/font.ttf"}
]})
Then: Image src "asset:logo", or Text style {"fontFamily": "Custom Font Family Name"}

REUSABLE FUNCTIONS (defineFunction) — AVOID for complex logic:
defineFunction exists but you should almost NEVER need it. All filtering, sorting, and lookups can be done INLINE using built-in functions composed in forEach values, props, and action ops.

INSTEAD OF defineFunction, use these patterns:
- Lookup tables: Store a map in the data model (e.g. /folderNames = {"notes":"Notes","work":"Work",...}), then use getField(/folderNames, selectedId)
- Inline filtering: Put filter/sort/if composition directly in the forEach value (see DATA MODEL PATTERNS below)
- Inline logic in actions: Put if/equals/concat directly in updateDataModel op values

Only use defineFunction for TRIVIALLY simple helpers (1-2 nested levels, like appendDigit for a calculator).
If a function body would have more than 3 nested functionCalls, do NOT use defineFunction — compose inline instead.

REUSABLE COMPONENTS (defineComponent):
Define component templates with parameters. Use {\"param\":\"name\"} for parameterization and \"$/path\" for scoped data:

a2ui_defineComponent({
  "name": "DigitButton",
  "params": ["digit", "label"],
  "components": [
    {"componentId":"_root","type":"Button","props":{
      "label":{"param":"label"},
      "onClick":{"action":{"functionCall":{"call":"updateDataModel","args":{"ops":[
        {"op":"replace","path":"/display","value":{"functionCall":{"name":"appendDigit","args":[{"param":"digit"}]}}}
      ]}}}}
    },"style":{"backgroundColor":"#333","cornerRadius":33,"width":66,"height":66}}
  ]
})

Instantiate in updateComponents: {"componentId":"btn7","useComponent":"DigitButton","args":{"digit":"7","label":"7"}}

Rules:
- Definition must have exactly one _root component (becomes the instance ID)
- Other IDs become {instanceId}_{originalId} (e.g. btn7__label)
- \"$/path\" is replaced with scope prefix (default: /instanceId)
- Use explicit scope for shared state: {"scope":"/calc1"}

DATA MODEL PATTERNS (use these instead of defineFunction):

1. LOOKUP TABLES — store name-to-value maps in the data model, look up with getField:
   Data model: /folderNames = {"icloud":"iCloud","all-icloud":"All iCloud","notes":"Notes","work":"Work","personal":"Personal","trash":"Recently Deleted"}
   Usage: getField(/folderNames, /selectedFolderId) → returns the display name for the selected folder
   This replaces any need for a defineFunction with if/equals chains.

   FOLDER TREE DATA — include noteCount for badge display:
   /folders = [{"id":"icloud","name":"iCloud","icon":"icloud","children":[{"id":"all-icloud","name":"All iCloud","icon":"tray.full.fill","noteCount":4},{"id":"notes","name":"Notes","icon":"folder.fill","noteCount":2,"children":[]},{"id":"work","name":"Work","icon":"folder.fill","noteCount":1,"children":[{"id":"meetings","name":"Meetings","icon":"folder.fill","noteCount":1}]}]},{"id":"mac","name":"On My Mac","icon":"laptopcomputer","children":[{"id":"personal","name":"Personal","icon":"folder.fill","noteCount":1}]},{"id":"trash","name":"Recently Deleted","icon":"trash","children":[]}]
   Every leaf folder MUST have "noteCount" matching the number of notes in that folder (shown as badge via badgeKey:"noteCount").
   BADGE ACCURACY: When using badgeKey on OutlineView, the badge values in tree data MUST exactly match the count of items that would be shown when that node is selected. Cross-check: sum of leaf node badges under a parent should equal the parent's total. Miscounted badges confuse users and break trust.

2. INLINE FILTERING for note list (full pattern with search + folder + account handling):
   The forEach value should be a single composed functionCall:
   sort(
     if(not(equals(/searchQuery, "")),
       filterContainsAny(/notes, ["title","content"], /searchQuery),
       if(or(equals(/selectedFolderId,"icloud"), equals(/selectedFolderId,"all-icloud"), equals(/selectedFolderId,"mac")),
         filter(/notes, "account", if(equals(/selectedFolderId,"all-icloud"), "icloud", /selectedFolderId)),
         filter(/notes, "folderId", /selectedFolderId)
       )
     ),
     /sortKey,
     /sortDescending
   )
   This handles: search mode (filter ALL notes by query across title+content), account-level folders (filter by account), and leaf folders (filter by folderId). No defineFunction needed.

3. NOTE COUNT DISPLAY — use the same inline filter pattern wrapped in length() + toString():
   concat(toString(length(if(searchActive, filterContainsAny(...), if(accountFolder, filter(...account...), filter(...folderId...))))), " Notes")

4. EDITOR CONTENT LOOKUP — use find + getField inline:
   richContent: if(not(equals(/selectedNoteId, "")), getField(find(/notes, "id", /selectedNoteId), "content"), "")
   This replaces any "getSelectedNote" defineFunction.

5. NOTE SNIPPET CLEANUP — strip ALL markdown formatting for list previews:
   trim(substring(replace(replace(replace(replace(replace(replace(substringAfter(/note/content, "\n\n"), "\n", " "), "#", ""), "**", ""), "- [ ] ", ""), "- [x] ", ""), "- ", ""), 0, 50))
   Chain: substringAfter skips title, replace strips newlines/headings/bold/checkboxes/bullets, substring(0,50) truncates, trim cleans whitespace. IMPORTANT: strip "- [ ] " and "- [x] " BEFORE "- " to avoid partial matches.

6. DELETE TO TRASH — save previousFolderId before moving, for undo support:
   ops: [
     {op:"replace", path:"/notes", value: if(equals(/selectedFolderId,"trash"), remove(/notes,"id",/selectedNoteId), updateItem(updateItem(/notes,"id",/selectedNoteId,"previousFolderId",getField(find(/notes,"id",/selectedNoteId),"folderId")),"id",/selectedNoteId,"folderId","trash"))},
     {op:"replace", path:"/selectedNoteId", value: ""}
   ]
   When in trash folder, permanently remove. Otherwise, save current folderId as previousFolderId then set folderId to "trash".

COMPONENT PATTERNS:

1. Three-pane SplitView (sidebar + list + detail):
Use a SINGLE root SplitView with 3 children (NOT nested 2-pane SplitViews):
Sidebar: {"componentId":"sidebar","type":"Column","children":{"static":["folderTree"]},"props":{"align":"stretch"},"style":{"width":200}}
List pane: {"componentId":"noteListPane","type":"Column","children":{"static":["listHeader","noteCount","noteList"]},"props":{"align":"stretch"},"style":{"width":280}}
Editor pane: {"componentId":"editorPane","type":"Column","children":{"static":["editorDate","editor","emptyLabel"]},"props":{"align":"stretch"},"style":{"flexGrow":1}}
Root: {"componentId":"root","type":"SplitView","props":{"vertical":true,"dividerStyle":"thin","collapsedPane":{"functionCall":{"name":"if","args":[{"path":"/sidebarCollapsed"},0,-1]}}},"children":{"static":["sidebar","noteListPane","editorPane"]}}
— IMPORTANT: "vertical":true is REQUIRED for side-by-side panes. style.width goes on CHILD panes (sidebar=200, noteListPane=280), NOT on the SplitView. collapsedPane=0 collapses first pane, -1=none. Use "align":"stretch" on Column panes so children fill width.

2. OutlineView with nested tree data:
First set data: a2ui_updateDataModel ops=[{op:"add",path:"/folders",value:[{"id":"f1","name":"Notes","icon":"folder.fill","noteCount":3,"children":[{"id":"n1","name":"My Note","icon":"doc.text"}]}]}]
Then: {"componentId":"sidebar_tree","type":"OutlineView","props":{"outlineData":{"path":"/folders"},"labelKey":"name","childrenKey":"children","iconKey":"icon","idKey":"id","selectedId":{"path":"/selectedNoteId"},"badgeKey":"noteCount","dataBinding":"/selectedNoteId"}}
IMPORTANT: badgeKey must match the actual field name in the data (e.g. "noteCount" in data → badgeKey:"noteCount"). Use icon names with ".fill" suffix for folders (folder.fill, tray.full.fill).

3. RichTextEditor with format binding and onChange (REQUIRED to persist edits):
Set format state: a2ui_updateDataModel ops=[{op:"add",path:"/format",value:{"bold":false,"italic":false,"underline":false,"strikethrough":false}}]
Then: {"componentId":"editor","type":"RichTextEditor","props":{"richContent":{"functionCall":{"name":"getField","args":[{"functionCall":{"name":"find","args":[{"path":"/notes"},"id",{"path":"/selectedNoteId"}]}},"content"]}},"editable":true,"formatBinding":"/format","onChange":{"action":{"functionCall":{"call":"updateDataModel","args":{"ops":[{"op":"replace","path":"/notes","value":{"functionCall":{"name":"updateItem","args":[{"path":"/notes"},"id",{"path":"/selectedNoteId"},"content",{"path":"/_input"}]}}}]}}}}},"style":{"flexGrow":1}}
— formatBinding auto-updates /format/bold etc. as cursor moves. Toolbar buttons can read these to show active state.
— onChange fires on every edit with new content at /_input. Use updateItem to write it back to the notes array. Without onChange, edits are lost when switching notes.

4. Toolbar with format buttons and search (note double-nested action):
a2ui_updateToolbar({surfaceId:"main",items:[
  {"id":"sidebar","icon":"sidebar.leading","label":"Sidebar","bordered":true,"action":{"action":{"functionCall":{"call":"updateDataModel","args":{"ops":[{"op":"replace","path":"/sidebarCollapsed","value":{"functionCall":{"name":"not","args":[{"path":"/sidebarCollapsed"}]}}}]}}}}},
  {"separator":true},
  {"id":"delete","icon":"trash","label":"Delete","bordered":true,"action":{"action":{"functionCall":{"call":"updateDataModel","args":{"ops":[{"op":"replace","path":"/notes","value":{"functionCall":{"name":"remove","args":[{"path":"/notes"},"id",{"path":"/selectedNoteId"}]}}},{"op":"replace","path":"/selectedNoteId","value":""}]}}}}},
  {"flexible":true},
  {"id":"newNote","icon":"square.and.pencil","label":"New Note","bordered":true,"action":{"action":{"functionCall":{"call":"updateDataModel","args":{"ops":[...]}}}}},
  {"separator":true},
  {"id":"bold","icon":"bold","label":"Bold","standardAction":"toggleBoldface:","bordered":true,"selected":{"path":"/format/bold"}},
  {"id":"italic","icon":"italic","label":"Italic","standardAction":"toggleItalics:","bordered":true,"selected":{"path":"/format/italic"}},
  {"id":"underline","icon":"underline","label":"Underline","standardAction":"underline:","bordered":true,"selected":{"path":"/format/underline"}},
  {"id":"strikethrough","icon":"strikethrough","label":"Strikethrough","standardAction":"addStrikethrough:","bordered":true,"selected":{"path":"/format/strikethrough"}},
  {"flexible":true},
  {"id":"search","searchField":true,"label":"Search","dataBinding":"/searchQuery"}
]})
— Custom actions use {"action":{"action":{...}}} (double-nested). standardAction goes directly on the item (no action wrapper needed).

5. forEach dynamic list with FULL inline filter+sort+search (no defineFunction needed):
The forEach value handles search mode, account-level folders, and leaf folders — all inline:
{"componentId":"noteList","type":"List","style":{"flexGrow":1},"children":{"forEach":{"functionCall":{"name":"sort","args":[{"functionCall":{"name":"if","args":[{"functionCall":{"name":"not","args":[{"functionCall":{"name":"equals","args":[{"path":"/searchQuery"},""]}}]}},{"functionCall":{"name":"filterContainsAny","args":[{"path":"/notes"},["title","content"],{"path":"/searchQuery"}]}},{"functionCall":{"name":"if","args":[{"functionCall":{"name":"or","args":[{"functionCall":{"name":"equals","args":[{"path":"/selectedFolderId"},"icloud"]}},{"functionCall":{"name":"equals","args":[{"path":"/selectedFolderId"},"all-icloud"]}},{"functionCall":{"name":"equals","args":[{"path":"/selectedFolderId"},"mac"]}}]}},{"functionCall":{"name":"filter","args":[{"path":"/notes"},"account",{"functionCall":{"name":"if","args":[{"functionCall":{"name":"equals","args":[{"path":"/selectedFolderId"},"all-icloud"]}},"icloud",{"path":"/selectedFolderId"}]}}]}},{"functionCall":{"name":"filter","args":[{"path":"/notes"},"folderId",{"path":"/selectedFolderId"}]}}]}}]}},{"path":"/sortKey"},{"path":"/sortDescending"}]}},"templateId":"notePreview","itemVariable":"note"}}
Template (must be in SAME updateComponents call as the noteList above):
[{"componentId":"notePreview","type":"Column","props":{"gap":0,"padding":0,"onClick":{"action":{"functionCall":{"call":"updateDataModel","args":{"ops":[{"op":"replace","path":"/selectedNoteId","value":{"path":"/note/id"}}]}}}},"contextMenu":[{"id":"pinNote","label":{"functionCall":{"name":"if","args":[{"functionCall":{"name":"equals","args":[{"functionCall":{"name":"getField","args":[{"functionCall":{"name":"find","args":[{"path":"/notes"},"id",{"path":"/note/id"}]}},"pinned"]}},"true"]}},"Unpin Note","Pin Note"]}},"icon":"pin","action":{"action":{"functionCall":{"call":"updateDataModel","args":{"ops":[{"op":"replace","path":"/notes","value":{"functionCall":{"name":"updateItem","args":[{"path":"/notes"},"id",{"path":"/note/id"},"pinned",{"functionCall":{"name":"if","args":[{"functionCall":{"name":"equals","args":[{"functionCall":{"name":"getField","args":[{"functionCall":{"name":"find","args":[{"path":"/notes"},"id",{"path":"/note/id"}]}},"pinned"]}},"true"]}},"false","true"]}}]}}}]}}}}},{"separator":true},{"id":"delete","label":"Delete","icon":"trash","action":{"action":{"functionCall":{"call":"updateDataModel","args":{"ops":[{"op":"replace","path":"/notes","value":{"functionCall":{"name":"if","args":[{"functionCall":{"name":"equals","args":[{"path":"/selectedFolderId"},"trash"]}},{"functionCall":{"name":"remove","args":[{"path":"/notes"},"id",{"path":"/note/id"}]}},{"functionCall":{"name":"updateItem","args":[{"functionCall":{"name":"updateItem","args":[{"path":"/notes"},"id",{"path":"/note/id"},"previousFolderId",{"functionCall":{"name":"getField","args":[{"functionCall":{"name":"find","args":[{"path":"/notes"},"id",{"path":"/note/id"}]}},"folderId"]}}]}},"id",{"path":"/note/id"},"folderId","trash"]}}]}}}]}}}}}]},"style":{"backgroundColor":{"functionCall":{"name":"if","args":[{"functionCall":{"name":"equals","args":[{"path":"/note/id"},{"path":"/selectedNoteId"}]}},"#FBE5A2",""]}},"cornerRadius":8},"children":{"static":["noteContent"]}},
{"componentId":"noteContent","type":"Column","props":{"gap":2,"padding":10},"children":{"static":["noteTitle","noteSubRow"]}},
{"componentId":"noteTitle","type":"Text","props":{"content":{"path":"/note/title"},"variant":"body","maxLines":1},"style":{"fontWeight":"bold","fontSize":14}},
{"componentId":"noteSubRow","type":"Row","props":{"gap":6,"align":"center"},"children":{"static":["noteDate","noteSnippet"]}},
{"componentId":"noteDate","type":"Text","props":{"content":{"functionCall":{"name":"formatDateRelative","args":[{"path":"/note/modified"}]}},"variant":"caption","maxLines":1},"style":{"textColor":"#3C3C43","fontSize":12}},
{"componentId":"noteSnippet","type":"Text","props":{"content":{"functionCall":{"name":"trim","args":[{"functionCall":{"name":"substring","args":[{"functionCall":{"name":"replace","args":[{"functionCall":{"name":"replace","args":[{"functionCall":{"name":"replace","args":[{"functionCall":{"name":"replace","args":[{"functionCall":{"name":"substringAfter","args":[{"path":"/note/content"},"\n\n"]}},"\n"," "]}},"#",""]}},"**",""]}},"- ",""]}},"- [ ] ",""]},0,50]}}]}},"variant":"caption","maxLines":1},"style":{"flexGrow":1,"textColor":"#8E8E93","fontSize":12}}]
— Note the snippet: substringAfter skips the title line, then replace strips markdown (headings, bold, list markers), substring truncates, trim cleans whitespace.
— forEach value is a functionCall (sort wrapping if/filter chain), NOT a path + separate "filter" key. The filter key is not supported. Do NOT use defineFunction for this — compose inline.

6. Menu with Edit and custom actions (keyboard shortcuts REQUIRE a menu):
a2ui_updateMenu({surfaceId:"main",items:[
  {"id":"file","label":"File","children":[
    {"id":"newNote","label":"New Note","keyEquivalent":"n","action":{"action":{"functionCall":{"call":"updateDataModel","args":{"ops":[{"op":"replace","path":"/notes","value":{"functionCall":{"name":"append","args":[{"path":"/notes"},{"id":"new","title":"New Note","content":""}]}}}]}}}}},
    {"id":"deleteNote","label":"Delete Note","keyEquivalent":"\b","action":{"action":{"functionCall":{"call":"updateDataModel","args":{"ops":[{"op":"replace","path":"/notes","value":{"functionCall":{"name":"remove","args":[{"path":"/notes"},"id",{"path":"/selectedNoteId"}]}}},{"op":"replace","path":"/selectedNoteId","value":""}]}}}}},
    {"separator":true},
    {"id":"close","label":"Close","keyEquivalent":"w","standardAction":"performClose:"}
  ]},
  {"id":"edit","label":"Edit","children":[
    {"id":"undo","label":"Undo","keyEquivalent":"z","standardAction":"undo:"},
    {"id":"redo","label":"Redo","keyEquivalent":"Z","standardAction":"redo:"},
    {"separator":true},
    {"id":"cut","label":"Cut","keyEquivalent":"x","standardAction":"cut:"},
    {"id":"copy","label":"Copy","keyEquivalent":"c","standardAction":"copy:"},
    {"id":"paste","label":"Paste","keyEquivalent":"v","standardAction":"paste:"},
    {"id":"selectAll","label":"Select All","keyEquivalent":"a","standardAction":"selectAll:"}
  ]},
  {"id":"view","label":"View","children":[
    {"id":"toggleSidebar","label":"Show/Hide Sidebar","keyEquivalent":"s","keyModifiers":"option","action":{"action":{"functionCall":{"call":"updateDataModel","args":{"ops":[{"op":"replace","path":"/sidebarCollapsed","value":{"functionCall":{"name":"not","args":[{"path":"/sidebarCollapsed"}]}}}]}}}}},
    {"separator":true},
    {"id":"sortDateEdited","label":"Date Edited","keyEquivalent":"1","keyModifiers":"option","action":{"action":{"functionCall":{"call":"updateDataModel","args":{"ops":[{"op":"replace","path":"/sortKey","value":"modified"}]}}}}},
    {"id":"sortTitle","label":"Title","keyEquivalent":"3","keyModifiers":"option","action":{"action":{"functionCall":{"call":"updateDataModel","args":{"ops":[{"op":"replace","path":"/sortKey","value":"title"}]}}}}}
  ]},
  {"id":"format","label":"Format","children":[
    {"id":"bold","label":"Bold","keyEquivalent":"b","standardAction":"toggleBoldface:"},
    {"id":"italic","label":"Italic","keyEquivalent":"i","standardAction":"toggleItalics:"},
    {"id":"underline","label":"Underline","keyEquivalent":"u","standardAction":"underline:"}
  ]}
]})
— Standard actions use standardAction directly (no action wrapper). Custom actions use "action":{"action":{...}} (double-nested). The Edit menu is REQUIRED for Cmd+C/V/X/Z to work.
— Use keyModifiers:"option" for Opt+key shortcuts (e.g. Opt+S for sidebar toggle, Opt+1/2/3 for sort). Default modifier is Cmd.

WORKFLOW (order matters):
1. Call a2ui_defineComponent to register reusable component templates (optional, rarely needed)
2. Call a2ui_loadAssets if you need custom fonts or want to preload images (optional)
3. Call a2ui_createSurface to create a window (optionally with backgroundColor and padding=-1 for edge-to-edge)
4. Call a2ui_updateWindow to set minimum window size constraints (optional)
5. Call a2ui_updateDataModel to set initial data. IMPORTANT: include lookup tables (like /folderNames map) so you can use getField() instead of defineFunction if-chains.
6. Call a2ui_updateMenu to set up menus with standard actions (Undo/Redo/Cut/Copy/Paste/Select All) — REQUIRED for keyboard shortcuts in text fields and editors. Custom actions use double-nested format: "action":{"action":{"functionCall":{...}}}
7. Call a2ui_updateToolbar to add native toolbar buttons (optional but recommended). Custom actions use the same double-nested format.
8. Call a2ui_updateComponents MULTIPLE TIMES to build the component tree in batches (after menu/toolbar are ready). See COMPONENT BATCHING below. Each call returns layout coordinates of the rendered components — CHECK THEM for zero-width/zero-height issues.
9. Call a2ui_takeScreenshot ONCE to visually verify layout. Do NOT take multiple screenshots — one is enough.
10. Immediately call a2ui_test to write tests (REQUIRED — at least 5 tests covering: initial layout/children, data model state, component props with resolved content, count of forEach-expanded items like noteList, and click simulation to verify data model updates). After tests, you are DONE — do not loop back to screenshots.

DO NOT call a2ui_defineFunction for complex logic — use inline function composition and data model lookup tables instead (see DATA MODEL PATTERNS above).

COMPONENT TREE RULES:
- Every component needs a unique componentId
- Tree structure is defined ONLY by children.static on the parent container — this is what determines layout
- Every container (Row, Column, Card, List) MUST have "children": {"static": ["childId1", "childId2", ...]} listing ALL its direct children in order
- Leaf components (Text, Button, TextField, etc.) do not need children
- Do NOT rely on parentId — it is not used for tree construction
- Data binding: set dataBinding to a JSON Pointer (e.g. "/form/name") and the component auto-syncs with the data model

COMPONENT BATCHING (REQUIRED):
a2ui_updateComponents has merge semantics — each call adds or updates components by componentId. Use 1-2 calls (ideally 1 call with all components). Fewer batches = better layout coherence.

Batching strategy for a 3-pane layout:
- Call 1: Sidebar + middle pane (outline tree, list templates + forEach container, list header — templates MUST be in same call as their forEach)
- Call 2: Editor pane + root SplitView (editor, labels, pane wrapper, root)

CRITICAL: forEach templates and their parent container MUST be in the SAME batch. The engine resolves templates within each batch.

Example for a Notes app (3-pane SplitView):
  // Batch 1: Sidebar + note list WITH templates (must be same batch as forEach)
  a2ui_updateComponents({surfaceId:"notes", components:[
    {componentId:"folderTree", type:"OutlineView", props:{outlineData:{path:"/folders"}, ...}, style:{flexGrow:1}},
    {componentId:"sidebar", type:"Column", children:{static:["folderTree"]}, props:{align:"stretch"}, style:{width:200}},
    {componentId:"noteTitle", type:"Text", props:{content:{path:"/note/title"}, maxLines:1}, style:{fontWeight:"bold",fontSize:14}},
    {componentId:"noteSubRow", type:"Row", props:{gap:6,align:"center"}, children:{static:["noteDate","noteSnippet"]}},
    {componentId:"noteDate", type:"Text", props:{content:{functionCall:{name:"formatDateRelative",args:[{path:"/note/modified"}]}}}, style:{textColor:"#3C3C43",fontSize:12}},
    {componentId:"noteSnippet", type:"Text", props:{content:{functionCall:{name:"trim",args:[{functionCall:{name:"substring",args:[{functionCall:{name:"replace",args:[{functionCall:{name:"replace",args:[{functionCall:{name:"replace",args:[{functionCall:{name:"replace",args:[{functionCall:{name:"replace",args:[{functionCall:{name:"replace",args:[{functionCall:{name:"substringAfter",args:[{path:"/note/content"},"\n\n"]}},"\n"," "]}},"#",""]}},"**",""]}},"- [ ] ",""]}},"- [x] ",""]}},"- ",""]}},0,50]}}]}}, maxLines:1}, style:{flexGrow:1,textColor:"#8E8E93",fontSize:12}},
    {componentId:"noteContent", type:"Column", props:{gap:2,padding:10}, children:{static:["noteTitle","noteSubRow"]}},
    {componentId:"notePreview", type:"Column", props:{gap:0, onClick:{...}, contextMenu:[...]}, style:{backgroundColor:{functionCall:...},cornerRadius:8}, children:{static:["noteContent"]}},
    {componentId:"listTitle", type:"Text", props:{content:{functionCall:{name:"if",args:[{functionCall:{name:"not",args:[{functionCall:{name:"equals",args:[{path:"/searchQuery"},""]}}]}},{functionCall:{name:"concat",args:["Search: ",{path:"/searchQuery"}]}},{functionCall:{name:"getField",args:[{path:"/folderNames"},{path:"/selectedFolderId"}]}}]}}}, style:{flexGrow:1,fontWeight:"bold",fontSize:17}},
    {componentId:"listHeader", type:"Row", props:{gap:8,align:"center",padding:8}, children:{static:["listTitle"]}},
    {componentId:"noteCount", type:"Text", props:{content:{functionCall:{name:"concat",args:[{functionCall:{name:"toString",args:[...]}}, " Notes"]}}}, style:{textColor:"#8E8E93",textAlign:"center"}},
    {componentId:"noteList", type:"List", children:{forEach:{functionCall:...}, templateId:"notePreview", itemVariable:"note"}, style:{flexGrow:1}},
    {componentId:"noteListPane", type:"Column", children:{static:["listHeader","noteCount","noteList"]}, props:{align:"stretch"}, style:{width:280}}
  ]})
  // Batch 2: Editor + root SplitView (NO width on SplitView — it fills its parent)
  a2ui_updateComponents({surfaceId:"notes", components:[
    {componentId:"editorDate", type:"Text", props:{content:{functionCall:...}}, style:{textColor:"#8E8E93",textAlign:"center"}},
    {componentId:"editor", type:"RichTextEditor", props:{richContent:{functionCall:...}, editable:true, formatBinding:"/formatState", onChange:{...}}, style:{flexGrow:1}},
    {componentId:"emptyLabel", type:"Text", props:{content:{functionCall:{name:"if",args:[{functionCall:{name:"equals",args:[{path:"/selectedNoteId"},""]}}, "Select a note", ""]}}}, style:{textColor:"#8E8E93",fontSize:16}},
    {componentId:"editorPane", type:"Column", children:{static:["editorDate","editor","emptyLabel"]}, props:{align:"stretch"}, style:{flexGrow:1}},
    {componentId:"root", type:"SplitView", props:{vertical:true, dividerStyle:"thin", collapsedPane:{functionCall:{name:"if",args:[{path:"/sidebarCollapsed"},0,-1]}}}, children:{static:["sidebar","noteListPane","editorPane"]}}
  ]})

LAYOUT FEEDBACK:
Components are buffered across a2ui_updateComponents calls and rendered all at once when you call a2ui_takeScreenshot or a2ui_test. Each updateComponents call responds with the number of components buffered. Do NOT try to fix layout between batches — the final layout is only visible after all components are rendered.

SCREENSHOTS:
Call a2ui_takeScreenshot ONCE after all components are rendered. Do NOT call it multiple times — one screenshot is sufficient. After the screenshot, proceed directly to writing tests.

TESTING (REQUIRED — write at least 8 tests):
After building a UI, you MUST write tests using a2ui_test to verify correctness. Tests execute IMMEDIATELY and return real PASS/FAIL results. If a test fails, read the error message and fix the issue before continuing. Write tests covering ALL of these: (1) initial layout/children, (2) initial data model state, (3) component props with resolved content, (4) count of forEach-expanded items, (5) click simulation to verify data model updates, (6) resolved text content of key components.

forEach-expanded component IDs follow the pattern: {listId}_{templateId}_{index}
Example: if noteList has templateId "notePreview", the expanded components are:
  noteList_notePreview_0, noteList_notePreview_1, ... (and their children: noteList_noteTitle_0, noteList_noteDate_0, etc.)

Assertion types:
- component: Subset match on resolved props. {"assert":"component","componentId":"id","props":{"content":"Hello","variant":"h1"}}
  Optionally check type: {"assert":"component","componentId":"id","componentType":"Text"}
- dataModel: Check data model value. {"assert":"dataModel","path":"/name","value":"Alice"}
- children: Check child IDs in order. {"assert":"children","componentId":"parent","children":["c1","c2"]}
- notExists: Verify component does not exist. {"assert":"notExists","componentId":"id"}
- count: Check child count. {"assert":"count","componentId":"parent","count":3}
- action: Check that an action was fired. {"assert":"action","name":"submitForm","data":{"/name":"Alice"}}
- layout: Check computed layout (x, y, width, height). {"assert":"layout","componentId":"id","layout":{"width":200,"height":50}}
- style: Check computed style (fontName, fontSize, bold, italic, textColor, bgColor, hidden, opacity). {"assert":"style","componentId":"id","style":{"fontSize":13,"bold":true}}

Simulation:
- event: Trigger user interaction. {"simulate":"event","componentId":"field","event":"change","eventData":"Alice"}
  Events: change (TextField), click (Button), toggle (CheckBox), slide (Slider), select (ChoicePicker/Tabs), datechange (DateTimeInput)

Tests execute in file order. Side effects persist across tests. Actions reset per test.

REQUIRED test patterns (include ALL of these):
1. Layout test: {"assert":"children","componentId":"root","children":["sidebar","noteListPane","editorPane"]}
2. Data model test: {"assert":"dataModel","path":"/selectedFolderId","value":"all-icloud"}
3. Count test (forEach items): {"assert":"count","componentId":"noteList","count":4}
4. Resolved content test: {"assert":"component","componentId":"noteList_noteTitle_0","props":{"content":"Meeting Notes"}}
5. Click simulation: {"simulate":"event","componentId":"noteList_notePreview_1","event":"click"}, then {"assert":"dataModel","path":"/selectedNoteId","value":"n2"}
6. Editor content test: {"assert":"component","componentId":"editor","props":{"richContent":"# Meeting Notes\n\n..."}}
7. Resolved header test: {"assert":"component","componentId":"listTitle","props":{"content":"All iCloud"}}

CHANNELS (Inter-Process Communication):
Create named channels for processes to communicate via publish/subscribe.

1. a2ui_createChannel — create a channel with broadcast (all subscribers receive) or queue (round-robin to one subscriber) mode
2. a2ui_subscribe — register to receive values; specify targetPath to auto-write published values to the data model
3. a2ui_publish — send a value to a channel; value is written to /channels/{channelId}/value in the data model
4. a2ui_unsubscribe — stop receiving values
5. a2ui_deleteChannel — remove the channel and all subscriptions

Published values are always written to /channels/{channelId}/value, so components can bind directly:
  "dataBinding": "/channels/myChannel/value"

Subscribers can also specify a targetPath to receive values at a custom data model path.
When a process stops, its channel subscriptions are automatically cleaned up.

FINAL CONSTRAINTS (verify before finishing — violations break the app):
1. RichTextEditor: ONLY use richContent + onChange. NEVER add dataBinding (causes dual-write corruption).
2. Toolbar search items MUST include label and use the SAME dataBinding path as the forEach search filter.
3. OutlineView badge values MUST match actual data counts. Cross-check before submitting.
4. Do NOT add data model fields beyond what the app requires. Only add fields that are directly used by the UI.
5. Every menu item with a keyboard shortcut MUST have keyEquivalent set (e.g. "\b" for backspace/delete).
6. Component IDs, data model paths, and prop names must be consistent across menus, toolbar, components, and data model. A path mismatch silently breaks binding.

USER REQUEST: ` + userPrompt
}

// handleInspectLibrary runs nm on a dylib and returns its exported symbols as a JSON string.
func handleInspectLibrary(tc anyllm.ToolCall) string {
	var args struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
		return fmt.Sprintf("error: invalid arguments: %v", err)
	}
	if args.Path == "" {
		return "error: path is required"
	}

	// Use platform-appropriate tool to list exported symbols
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("nm", "-gU", args.Path)
	default:
		cmd = exec.Command("nm", "-D", "--defined-only", args.Path)
	}

	out, err := cmd.Output()
	if err != nil {
		return fmt.Sprintf("error: failed to inspect library: %v", err)
	}

	// Parse nm output: each line is "address type name"
	var symbols []string
	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}
		sym := fields[2]
		// Strip leading underscore on macOS
		if runtime.GOOS == "darwin" && strings.HasPrefix(sym, "_") {
			sym = sym[1:]
		}
		symbols = append(symbols, sym)
	}

	result, _ := json.Marshal(map[string]any{
		"path":    args.Path,
		"symbols": symbols,
		"count":   len(symbols),
	})
	return string(result)
}

// handleGetLogs queries the application log and returns matching entries as text.
func handleGetLogs(tc anyllm.ToolCall) string {
	var args struct {
		Level     string `json:"level"`
		Component string `json:"component"`
		SurfaceID string `json:"surface_id"`
		Pattern   string `json:"pattern"`
		Limit     int    `json:"limit"`
		Offset    int    `json:"offset"`
	}
	if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
		return fmt.Sprintf("error: invalid arguments: %v", err)
	}

	minLevel := jlog.LevelInfo
	if args.Level != "" {
		minLevel = jlog.ParseLevel(args.Level)
	}

	result := jlog.Query(jlog.QueryOpts{
		MinLevel:  minLevel,
		Component: args.Component,
		Surface:   args.SurfaceID,
		Pattern:   args.Pattern,
		Limit:     args.Limit,
		Offset:    args.Offset,
	})

	// Format as text for the LLM
	var b strings.Builder
	fmt.Fprintf(&b, "Log entries: %d of %d total\n\n", len(result.Entries), result.Total)
	for _, e := range result.Entries {
		if e.Surface != "" {
			fmt.Fprintf(&b, "%s %s [%s/%s] %s\n", e.Time.Format("15:04:05.000"), e.LevelStr, e.Component, e.Surface, e.Message)
		} else {
			fmt.Fprintf(&b, "%s %s [%s] %s\n", e.Time.Format("15:04:05.000"), e.LevelStr, e.Component, e.Message)
		}
	}
	return b.String()
}

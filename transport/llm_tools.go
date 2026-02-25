package transport

import (
	"encoding/json"
	"fmt"
	"jview/jlog"
	"jview/protocol"
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
				Description: "Create or update UI components on a surface. Components form a tree: containers (Row, Column, Card, List) declare their children via children.static, leaf components (Text, Button, TextField, etc.) do not. Each component has a unique componentId. Props vary by type.",
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
									"type":        map[string]any{"type": "string", "enum": []string{"Text", "Row", "Column", "Card", "Button", "TextField", "CheckBox", "Slider", "Image", "Icon", "Divider", "List", "Tabs", "Modal", "ChoicePicker", "DateTimeInput"}},
									"children":    map[string]any{"type": "object", "description": "Tree structure: {\"static\": [\"childId1\", \"childId2\"]}. Required on containers."},
									"props":       map[string]any{"type": "object"},
									"style": map[string]any{
										"type":        "object",
										"description": "Visual styling overrides",
										"properties": map[string]any{
											"backgroundColor": map[string]any{"type": "string", "description": "Background color (#RRGGBB)"},
											"textColor":       map[string]any{"type": "string", "description": "Text color (#RRGGBB)"},
											"cornerRadius":    map[string]any{"type": "number", "description": "Corner radius in points"},
											"width":           map[string]any{"type": "number", "description": "Fixed width in points"},
											"height":          map[string]any{"type": "number", "description": "Fixed height in points"},
											"fontSize":        map[string]any{"type": "number", "description": "Font size in points"},
											"fontWeight":      map[string]any{"type": "string", "enum": []string{"bold", "semibold", "medium", "light"}},
											"fontFamily":      map[string]any{"type": "string", "description": "Font family name (system or registered via a2ui_loadAssets)"},
											"textAlign":       map[string]any{"type": "string", "enum": []string{"left", "center", "right"}},
											"opacity":         map[string]any{"type": "number", "description": "Opacity 0.0-1.0"},
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
		{
			Type: "function",
			Function: anyllm.Function{
				Name:        "a2ui_deleteSurface",
				Description: "Close and destroy a UI surface (window).",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"surfaceId": map[string]any{"type": "string"},
					},
					"required": []string{"surfaceId"},
				},
			},
		},
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
									"separator":      map[string]any{"type": "boolean", "description": "True for a separator line"},
									"standardAction": map[string]any{"type": "string", "description": "AppKit selector string (e.g. \"copy:\", \"paste:\", \"undo:\", \"selectAll:\"). Routes through responder chain."},
									"action":         map[string]any{"type": "object", "description": "Custom action (same structure as button onClick)"},
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
				Description: "Define a reusable function with parameters. The body is a JSON expression tree (using functionCall, path refs, and literals) where {\"param\":\"name\"} nodes get replaced with argument values at call time. User-defined functions can be used anywhere a built-in function can: in value expressions, in button onClick actions, etc.",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"name":   map[string]any{"type": "string", "description": "Function name (must not conflict with built-in functions)"},
						"params": map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Parameter names (positional)"},
						"body":   map[string]any{"description": "JSON expression tree with {\"param\":\"name\"} placeholders"},
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

// systemPrompt returns the system message that teaches the LLM about A2UI.
func systemPrompt(userPrompt string) string {
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

Surface-level styling:
- backgroundColor on createSurface sets the window background color
- padding on createSurface sets root inset (default 20, use -1 for edge-to-edge)

Layout tip: use justify:"fillEqually" on Row/Column to make all children equal-width/height.

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

REUSABLE FUNCTIONS (defineFunction):
Define reusable expression trees with parameters. Use {\"param\":\"name\"} as placeholders in the body:

a2ui_defineFunction({
  "name": "appendDigit",
  "params": ["digit"],
  "body": {"functionCall":{"name":"if","args":[
    {"functionCall":{"name":"or","args":[{"path":"/clearOnInput"},{"functionCall":{"name":"equals","args":[{"path":"/display"},"0"]}}]}},
    {"param":"digit"},
    {"functionCall":{"name":"concat","args":[{"path":"/display"},{"param":"digit"}]}}
  ]}}
})

Then call: {"functionCall":{"name":"appendDigit","args":["7"]}}
User functions are checked after built-ins, before FFI. They can call other user functions.

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

WORKFLOW:
1. Call a2ui_defineFunction to register reusable functions (optional)
2. Call a2ui_defineComponent to register reusable component templates (optional)
3. Call a2ui_loadAssets if you need custom fonts or want to preload images (optional)
4. Call a2ui_createSurface to create a window (optionally with backgroundColor and padding)
5. Call a2ui_updateDataModel to set initial data (if needed)
6. Call a2ui_updateComponents to create the component tree (can use useComponent for defined templates)
7. Call a2ui_updateMenu to set up an Edit menu with standard actions (Undo/Redo/Cut/Copy/Paste/Select All) — this is REQUIRED for keyboard shortcuts to work in text fields and editors
8. When the user interacts (clicks a button), you'll receive the action details. Respond by updating data or components.

COMPONENT TREE RULES:
- Every component needs a unique componentId
- Tree structure is defined ONLY by children.static on the parent container — this is what determines layout
- Every container (Row, Column, Card, List) MUST have "children": {"static": ["childId1", "childId2", ...]} listing ALL its direct children in order
- Leaf components (Text, Button, TextField, etc.) do not need children
- Do NOT rely on parentId — it is not used for tree construction
- Data binding: set dataBinding to a JSON Pointer (e.g. "/form/name") and the component auto-syncs with the data model

TESTING:
After building a UI, write tests using a2ui_test to verify correctness. Tests run headlessly.

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

package transport

import (
	"encoding/json"
	"fmt"
	"jview/protocol"
	"strings"

	anyllm "github.com/mozilla-ai/any-llm-go"
)

// a2uiTools returns the 5 A2UI tool definitions for the LLM.
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
						"surfaceId": map[string]any{"type": "string", "description": "Unique surface identifier"},
						"title":     map[string]any{"type": "string", "description": "Window title"},
						"width":     map[string]any{"type": "integer", "description": "Window width in points (default 800)"},
						"height":    map[string]any{"type": "integer", "description": "Window height in points (default 600)"},
					},
					"required": []string{"surfaceId", "title"},
				},
			},
		},
		{
			Type: "function",
			Function: anyllm.Function{
				Name:        "a2ui_updateComponents",
				Description: "Create or update UI components on a surface. Components form a tree: containers (Row, Column, Card, List) have children, leaf components (Text, Button, TextField, etc.) do not. Each component has a unique componentId and optional parentId. Props vary by type.",
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
									"type":        map[string]any{"type": "string", "enum": []string{"Text", "Row", "Column", "Card", "Button", "TextField", "CheckBox", "Slider", "Image", "Icon", "Divider", "List", "ChoicePicker", "DateTimeInput"}},
									"parentId":    map[string]any{"type": "string"},
									"children":    map[string]any{"type": "object"},
									"props":       map[string]any{"type": "object"},
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
	}
}

// toolCallToMessage converts a tool call into a protocol.Message by injecting the "type" field
// and parsing through the standard protocol parser path.
func toolCallToMessage(tc anyllm.ToolCall) (*protocol.Message, error) {
	name := tc.Function.Name
	if !strings.HasPrefix(name, "a2ui_") {
		return nil, fmt.Errorf("unknown tool call: %s", name)
	}
	msgType := name[5:] // strip "a2ui_" prefix

	// Parse the arguments JSON
	var args map[string]json.RawMessage
	if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
		return nil, fmt.Errorf("parse tool call args: %w", err)
	}

	// Inject the "type" field
	typeBytes, _ := json.Marshal(msgType)
	args["type"] = typeBytes

	// Re-serialize to a complete JSONL line
	line, err := json.Marshal(args)
	if err != nil {
		return nil, fmt.Errorf("serialize tool call: %w", err)
	}

	// Parse through the standard protocol parser
	parser := protocol.NewParser(strings.NewReader(string(line)))
	return parser.Next()
}

// systemPrompt returns the system message that teaches the LLM about A2UI.
func systemPrompt(userPrompt string) string {
	return `You are a UI builder. You create native macOS user interfaces using the A2UI protocol via tool calls.

AVAILABLE COMPONENTS:
- Text: Display text. Props: content (string), variant ("h1"|"h2"|"h3"|"body"|"caption")
- Row: Horizontal layout. Props: gap (int), padding (int), justify, align
- Column: Vertical layout. Props: gap (int), padding (int), justify, align
- Card: Titled container. Props: title (string), subtitle (string)
- Button: Clickable button. Props: label (string), style ("primary"|"secondary"|"destructive"), onClick.action.type ("serverAction"), onClick.action.name (string), onClick.action.dataRefs (array of JSON pointers)
- TextField: Text input. Props: placeholder (string), value (string), dataBinding (JSON pointer)
- CheckBox: Toggle. Props: label (string), checked (bool), dataBinding (JSON pointer)
- Slider: Range input. Props: min (number), max (number), step (number), sliderValue (number), dataBinding (JSON pointer)
- Image: Display image. Props: src (URL string), alt (string), width (int), height (int)
- Icon: SF Symbol. Props: name (string), size (int)
- Divider: Visual separator. No props needed.
- List: Scrollable list container.
- ChoicePicker: Dropdown/selection. Props: options (array of {value, label}), dataBinding (JSON pointer), mutuallyExclusive (bool)
- DateTimeInput: Date/time picker. Props: enableDate (bool), enableTime (bool), dataBinding (JSON pointer)

WORKFLOW:
1. Call a2ui_createSurface to create a window
2. Call a2ui_updateDataModel to set initial data (if needed)
3. Call a2ui_updateComponents to create the component tree
4. When the user interacts (clicks a button), you'll receive the action details. Respond by updating data or components.

COMPONENT TREE RULES:
- Every component needs a unique componentId
- Containers (Row, Column, Card, List) use children.static (array of child componentIds)
- Set parentId on children to point to their container
- Data binding: set dataBinding to a JSON Pointer (e.g. "/form/name") and the component auto-syncs with the data model

USER REQUEST: ` + userPrompt
}

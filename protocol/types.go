package protocol

import "encoding/json"

// MessageType identifies A2UI message kinds.
type MessageType string

const (
	MsgCreateSurface    MessageType = "createSurface"
	MsgDeleteSurface    MessageType = "deleteSurface"
	MsgUpdateComponents MessageType = "updateComponents"
	MsgUpdateDataModel  MessageType = "updateDataModel"
	MsgSetTheme         MessageType = "setTheme"
	MsgTest             MessageType = "test"
	MsgLoadLibrary      MessageType = "loadLibrary"
	MsgLoadAssets       MessageType = "loadAssets"
	MsgDefineFunction   MessageType = "defineFunction"
	MsgDefineComponent  MessageType = "defineComponent"
	MsgInclude          MessageType = "include"
	MsgCreateProcess    MessageType = "createProcess"
	MsgStopProcess      MessageType = "stopProcess"
	MsgSendToProcess    MessageType = "sendToProcess"
	MsgCreateChannel    MessageType = "createChannel"
	MsgDeleteChannel    MessageType = "deleteChannel"
	MsgPublish          MessageType = "publish"
	MsgSubscribe        MessageType = "subscribe"
	MsgUnsubscribe      MessageType = "unsubscribe"
	MsgUpdateMenu       MessageType = "updateMenu"
)

// TestMessage defines a test case with a sequence of assert/simulate steps.
type TestMessage struct {
	Type      MessageType `json:"type"`
	SurfaceID string      `json:"surfaceId"`
	Name      string      `json:"name"`
	Steps     []TestStep  `json:"steps"`
}

// TestStep is a single assertion or simulation within a test.
type TestStep struct {
	// Discriminator: "component", "dataModel", "children", "notExists", "count", "action", "layout", "style"
	Assert string `json:"assert,omitempty"`
	// Discriminator: "event"
	Simulate string `json:"simulate,omitempty"`

	// Common
	ComponentID string `json:"componentId,omitempty"`

	// assert=component: subset match on resolved props
	Props map[string]interface{} `json:"props,omitempty"`
	// assert=component: check component type
	ComponentType string `json:"componentType,omitempty"`

	// assert=dataModel
	Path  string      `json:"path,omitempty"`
	Value interface{} `json:"value"`

	// assert=children
	Children []string `json:"children,omitempty"`

	// assert=count
	Count int `json:"count,omitempty"`

	// assert=action
	ActionName string                 `json:"name,omitempty"`
	ActionData map[string]interface{} `json:"data,omitempty"`

	// assert=layout: check computed layout properties
	Layout map[string]interface{} `json:"layout,omitempty"`

	// assert=style: check computed style properties
	Style map[string]interface{} `json:"style,omitempty"`

	// simulate=event
	Event     string `json:"event,omitempty"`
	EventData string `json:"eventData,omitempty"`
}

// Envelope wraps every A2UI JSONL line.
type Envelope struct {
	Type      MessageType     `json:"type"`
	SurfaceID string          `json:"surfaceId,omitempty"`
	Payload   json.RawMessage `json:"-"`
}

func (e *Envelope) UnmarshalJSON(data []byte) error {
	type alias Envelope
	var a alias
	if err := json.Unmarshal(data, &a); err != nil {
		return err
	}
	*e = Envelope(a)
	e.Payload = data
	return nil
}

// CreateSurface tells the renderer to open a new surface (window).
type CreateSurface struct {
	Type            MessageType `json:"type"`
	SurfaceID       string      `json:"surfaceId"`
	Title           string      `json:"title"`
	Width           int         `json:"width,omitempty"`
	Height          int         `json:"height,omitempty"`
	BackgroundColor string      `json:"backgroundColor,omitempty"`
	Padding         int         `json:"padding,omitempty"`
}

// DeleteSurface removes a surface.
type DeleteSurface struct {
	Type      MessageType `json:"type"`
	SurfaceID string      `json:"surfaceId"`
}

// UpdateComponents sends a batch of component definitions.
type UpdateComponents struct {
	Type       MessageType `json:"type"`
	SurfaceID  string      `json:"surfaceId"`
	Components []Component `json:"components"`
}

// DataModelOp is a single data model operation.
type DataModelOp struct {
	Op    string      `json:"op"`    // "add", "replace", "remove"
	Path  string      `json:"path"`  // JSON Pointer
	Value interface{} `json:"value,omitempty"`
}

// UpdateDataModel applies operations to a surface's data model.
type UpdateDataModel struct {
	Type      MessageType   `json:"type"`
	SurfaceID string        `json:"surfaceId"`
	Ops       []DataModelOp `json:"ops"`
}

// SetTheme changes the visual theme.
type SetTheme struct {
	Type      MessageType `json:"type"`
	SurfaceID string      `json:"surfaceId"`
	Theme     string      `json:"theme"` // "light", "dark", "system"
}

// LoadLibrary dynamically loads a native library and registers its functions.
type LoadLibrary struct {
	Type      MessageType       `json:"type"`
	Path      string            `json:"path"`
	Prefix    string            `json:"prefix"`
	Functions []LoadLibraryFunc `json:"functions"`
}

// LoadAssets declares assets (fonts, images, audio, video) by alias.
type LoadAssets struct {
	Type   MessageType  `json:"type"`
	Assets []AssetEntry `json:"assets"`
}

// AssetEntry is a single asset declaration within a loadAssets message.
type AssetEntry struct {
	Alias string `json:"alias"`
	Kind  string `json:"kind"` // "font", "image", "audio", "video"
	Src   string `json:"src"`
}

// LoadLibraryFunc declares a single function to load from a native library.
type LoadLibraryFunc struct {
	Name       string   `json:"name"`
	Symbol     string   `json:"symbol"`
	ReturnType string   `json:"returnType,omitempty"`
	ParamTypes []string `json:"paramTypes,omitempty"`
	FixedArgs  int      `json:"fixedArgs,omitempty"`
}

// DefineFunction registers a reusable function with parametric body.
type DefineFunction struct {
	Type   MessageType `json:"type"`
	Name   string      `json:"name"`
	Params []string    `json:"params"`
	Body   interface{} `json:"body"`
}

// DefineComponent registers a reusable component template with parameters.
type DefineComponent struct {
	Type       MessageType       `json:"type"`
	Name       string            `json:"name"`
	Params     []string          `json:"params"`
	Components []json.RawMessage `json:"components"`
}

// Include directs the transport to inline another JSONL file.
type Include struct {
	Type MessageType `json:"type"`
	Path string      `json:"path"`
}

// UpdateMenu defines a menu bar for a surface's window.
type UpdateMenu struct {
	Type      MessageType `json:"type"`
	SurfaceID string      `json:"surfaceId"`
	Items     []MenuItem  `json:"items"`
}

// MenuItem is a single menu or menu item.
type MenuItem struct {
	ID             string       `json:"id,omitempty"`
	Label          string       `json:"label,omitempty"`
	KeyEquivalent  string       `json:"keyEquivalent,omitempty"`
	Separator      bool         `json:"separator,omitempty"`
	StandardAction string       `json:"standardAction,omitempty"`
	Action         *EventAction `json:"action,omitempty"`
	Children       []MenuItem   `json:"children,omitempty"`
}

package protocol

// EventAction is what happens when a user interacts with a component.
type EventAction struct {
	Action *Action `json:"action,omitempty"`
}

// Action describes an interaction outcome — either a server-bound event, a client-side function call,
// or a standard AppKit action routed through the responder chain.
type Action struct {
	Event          *EventDef       `json:"event,omitempty"`
	FunctionCall   *ActionFuncCall `json:"functionCall,omitempty"`
	StandardAction string          `json:"standardAction,omitempty"`
}

// EventDef is a server-bound event with optional data references.
type EventDef struct {
	Name      string                 `json:"name"`
	Context   map[string]interface{} `json:"context,omitempty"`
	DataRefs  []string               `json:"dataRefs,omitempty"`
	ProcessID string                 `json:"processId,omitempty"`
}

// ActionFuncCall is a client-side function call (e.g., updateDataModel).
type ActionFuncCall struct {
	Call string      `json:"call"`
	Args interface{} `json:"args,omitempty"`
}

// FunctionCall represents a built-in function evaluation (used in dynamic values).
type FunctionCall struct {
	Name string        `json:"name"`
	Args []interface{} `json:"args"`
}

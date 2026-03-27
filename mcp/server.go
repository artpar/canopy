package mcp

import (
	"context"
	"encoding/json"
	"jview/engine"
	"jview/jlog"
	"jview/protocol"
	"jview/renderer"
	"sync"
)

// PendingAction represents a user action queued for polling by external clients.
type PendingAction struct {
	SurfaceID   string                 `json:"surface_id"`
	ComponentID string                 `json:"component_id,omitempty"`
	Event       string                 `json:"event"`
	Data        map[string]interface{} `json:"data,omitempty"`
}

// Server is an MCP server that wraps a jview Session and Renderer.
type Server struct {
	sess  *engine.Session
	rend  renderer.Renderer
	disp  renderer.Dispatcher
	pm    *engine.ProcessManager
	cm    *engine.ChannelManager
	tools map[string]toolHandler

	// Component reference text for resources/read
	componentRef string

	// OnToolCall is called whenever a tool is invoked, with the tool name.
	// Used to update splash status during Claude Code generation.
	OnToolCall func(toolName string)

	// Action queue for external transports (e.g. Claude Code)
	actionMu sync.Mutex
	actions  []PendingAction
	actionCh chan struct{} // signaled when actions are pushed, cap 1
}

type toolHandler struct {
	def     Tool
	handler func(json.RawMessage) *ToolCallResult
}

// ServerOption configures an MCP server.
type ServerOption func(*Server)

// WithProcessManager attaches a process manager to the MCP server.
func WithProcessManager(pm *engine.ProcessManager) ServerOption {
	return func(s *Server) { s.pm = pm }
}

// WithChannelManager attaches a channel manager to the MCP server.
func WithChannelManager(cm *engine.ChannelManager) ServerOption {
	return func(s *Server) { s.cm = cm }
}

// WithComponentReference sets the A2UI protocol reference text
// that is served via MCP resources/read.
func WithComponentReference(ref string) ServerOption {
	return func(s *Server) { s.componentRef = ref }
}

// NewServer creates a new MCP server with optional managers.
func NewServer(sess *engine.Session, rend renderer.Renderer, disp renderer.Dispatcher, opts ...ServerOption) *Server {
	s := &Server{
		sess:     sess,
		rend:     rend,
		disp:     disp,
		tools:    make(map[string]toolHandler),
		actionCh: make(chan struct{}, 1),
	}
	for _, opt := range opts {
		opt(s)
	}
	s.registerTools()
	return s
}

// Run starts the MCP server, reading from r and writing to w.
// Blocks until context is cancelled or EOF on reader.
func (s *Server) Run(ctx context.Context, transport *StdioTransport) error {
	return MessageLoop(ctx, transport, s.handle)
}

func (s *Server) handle(req *Request) *Response {
	switch req.Method {
	case MethodInitialize:
		return s.handleInitialize()
	case MethodInitialized:
		return nil // notification, no response
	case MethodPing:
		return s.resultResponse(struct{}{})
	case MethodToolsList:
		return s.handleToolsList()
	case MethodToolsCall:
		return s.handleToolsCall(req)
	case MethodResourcesList:
		return s.handleResourcesList()
	case MethodResourcesRead:
		return s.handleResourcesRead(req)
	case MethodCancelled:
		return nil
	default:
		return &Response{
			Error: &ResponseError{
				Code:    MethodNotFound,
				Message: "method not found: " + req.Method,
			},
		}
	}
}

func (s *Server) handleInitialize() *Response {
	caps := ServerCapabilities{
		Tools: &ToolsCapability{},
	}
	if s.componentRef != "" {
		caps.Resources = &ResourcesCapability{}
	}
	result := InitializeResult{
		ProtocolVersion: ProtocolVersion,
		Capabilities:    caps,
		ServerInfo: ServerInfo{
			Name:    "jview",
			Version: "0.1.0",
		},
	}
	return s.resultResponse(result)
}

func (s *Server) handleResourcesList() *Response {
	var resources []Resource
	if s.componentRef != "" {
		resources = append(resources, Resource{
			URI:         "a2ui://reference",
			Name:        "A2UI Protocol Reference",
			Description: "Complete A2UI protocol reference for building native macOS UIs",
			MimeType:    "text/plain",
		})
	}
	return s.resultResponse(ResourcesListResult{Resources: resources})
}

func (s *Server) handleResourcesRead(req *Request) *Response {
	var params ResourcesReadParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return &Response{
			Error: &ResponseError{
				Code:    InvalidParams,
				Message: "invalid params: " + err.Error(),
			},
		}
	}
	if params.URI != "a2ui://reference" || s.componentRef == "" {
		return &Response{
			Error: &ResponseError{
				Code:    InvalidParams,
				Message: "resource not found: " + params.URI,
			},
		}
	}
	return s.resultResponse(ResourcesReadResult{
		Contents: []ResourceContent{{
			URI:      "a2ui://reference",
			MimeType: "text/plain",
			Text:     s.componentRef,
		}},
	})
}

func (s *Server) handleToolsList() *Response {
	var tools []Tool
	for _, th := range s.tools {
		tools = append(tools, th.def)
	}
	return s.resultResponse(ToolsListResult{Tools: tools})
}

func (s *Server) handleToolsCall(req *Request) *Response {
	var params ToolCallParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return &Response{
			Error: &ResponseError{
				Code:    InvalidParams,
				Message: "invalid params: " + err.Error(),
			},
		}
	}

	jlog.Infof("mcp", "", "tools/call: name=%s args=%s", params.Name, truncate(string(params.Arguments), 200))

	if s.OnToolCall != nil {
		s.OnToolCall(params.Name)
	}

	th, ok := s.tools[params.Name]
	if !ok {
		jlog.Warnf("mcp", "", "tools/call: unknown tool %q", params.Name)
		return &Response{
			Error: &ResponseError{
				Code:    MethodNotFound,
				Message: "unknown tool: " + params.Name,
			},
		}
	}

	result := th.handler(params.Arguments)
	jlog.Infof("mcp", "", "tools/call: %s done isError=%v", params.Name, result.IsError)
	return s.resultResponse(result)
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

func (s *Server) resultResponse(v any) *Response {
	data, err := json.Marshal(v)
	if err != nil {
		jlog.Errorf("mcp", "", "marshal error: %v", err)
		return &Response{
			Error: &ResponseError{
				Code:    InternalError,
				Message: "internal error: " + err.Error(),
			},
		}
	}
	return &Response{Result: data}
}

func (s *Server) register(name, description string, schema json.RawMessage, fn func(json.RawMessage) *ToolCallResult) {
	s.tools[name] = toolHandler{
		def: Tool{
			Name:        name,
			Description: description,
			InputSchema: schema,
		},
		handler: fn,
	}
}

// ToolNames returns the registered tool names.
func (s *Server) ToolNames() []string {
	names := make([]string, 0, len(s.tools))
	for name := range s.tools {
		names = append(names, name)
	}
	return names
}

// SendMessage allows the MCP client to send A2UI JSONL messages to the session.
func (s *Server) SendMessage(raw json.RawMessage) error {
	msg, err := protocol.ParseLine(raw)
	if err != nil {
		return err
	}
	jlog.Infof("mcp", "", "SendMessage: type=%s", msg.Type)
	s.sess.HandleMessage(msg)
	// Flush any buffered components so subsequent MCP queries see the result
	jlog.Infof("mcp", "", "SendMessage: flushing pending components")
	s.sess.FlushPendingComponents()
	return nil
}

// PushAction queues a user action for polling by external clients.
// Thread-safe. Drops oldest actions if queue exceeds 100.
func (s *Server) PushAction(action PendingAction) {
	s.actionMu.Lock()
	s.actions = append(s.actions, action)
	if len(s.actions) > 100 {
		s.actions = s.actions[len(s.actions)-100:]
	}
	s.actionMu.Unlock()

	// Signal waiters (non-blocking)
	select {
	case s.actionCh <- struct{}{}:
	default:
	}
}

// DrainActions returns and clears all queued actions.
func (s *Server) DrainActions() []PendingAction {
	s.actionMu.Lock()
	defer s.actionMu.Unlock()
	if len(s.actions) == 0 {
		return nil
	}
	result := s.actions
	s.actions = nil
	return result
}

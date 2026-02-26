package mcp

import (
	"context"
	"encoding/json"
	"jview/engine"
	"jview/jlog"
	"jview/protocol"
	"jview/renderer"
)

// Server is an MCP server that wraps a jview Session and Renderer.
type Server struct {
	sess  *engine.Session
	rend  renderer.Renderer
	disp  renderer.Dispatcher
	pm    *engine.ProcessManager
	cm    *engine.ChannelManager
	tools map[string]toolHandler
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

// NewServer creates a new MCP server with optional managers.
func NewServer(sess *engine.Session, rend renderer.Renderer, disp renderer.Dispatcher, opts ...ServerOption) *Server {
	s := &Server{
		sess:  sess,
		rend:  rend,
		disp:  disp,
		tools: make(map[string]toolHandler),
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
		return s.handleInitialize(req)
	case MethodInitialized:
		return nil // notification, no response
	case MethodPing:
		return s.resultResponse(struct{}{})
	case MethodToolsList:
		return s.handleToolsList()
	case MethodToolsCall:
		return s.handleToolsCall(req)
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

func (s *Server) handleInitialize(req *Request) *Response {
	result := InitializeResult{
		ProtocolVersion: ProtocolVersion,
		Capabilities: ServerCapabilities{
			Tools: &ToolsCapability{},
		},
		ServerInfo: ServerInfo{
			Name:    "jview",
			Version: "0.1.0",
		},
	}
	return s.resultResponse(result)
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

	th, ok := s.tools[params.Name]
	if !ok {
		return &Response{
			Error: &ResponseError{
				Code:    MethodNotFound,
				Message: "unknown tool: " + params.Name,
			},
		}
	}

	result := th.handler(params.Arguments)
	return s.resultResponse(result)
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
	s.sess.HandleMessage(msg)
	// Flush any buffered components so subsequent MCP queries see the result
	s.sess.FlushPendingComponents()
	return nil
}

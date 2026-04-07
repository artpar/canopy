package mcp

import (
	"bytes"
	"encoding/json"
	"canopy/engine"
	"canopy/renderer"
	"strings"
	"testing"
)

func callTool(srv *Server, name string, args string) *ToolCallResult {
	raw := json.RawMessage(args)
	th, ok := srv.tools[name]
	if !ok {
		return errorResult("tool not found: " + name)
	}
	return th.handler(raw)
}

func newServer() *Server {
	mock := renderer.NewMockRenderer()
	disp := &renderer.MockDispatcher{}
	sess := engine.NewSession(mock, disp)
	return NewServer(sess, mock, disp)
}

func TestInitialize(t *testing.T) {
	srv := newServer()

	resp := srv.handle(&Request{
		Method: MethodInitialize,
		Params: json.RawMessage(`{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}`),
	})

	if resp.Error != nil {
		t.Fatalf("initialize error: %s", resp.Error.Message)
	}

	var result InitializeResult
	json.Unmarshal(resp.Result, &result)

	if result.ProtocolVersion != ProtocolVersion {
		t.Errorf("protocol version = %q, want %q", result.ProtocolVersion, ProtocolVersion)
	}
	if result.ServerInfo.Name != "jview" {
		t.Errorf("server name = %q, want %q", result.ServerInfo.Name, "jview")
	}
}

func TestToolsList(t *testing.T) {
	srv := newServer()

	resp := srv.handle(&Request{Method: MethodToolsList})
	if resp.Error != nil {
		t.Fatalf("tools/list error: %s", resp.Error.Message)
	}

	var result ToolsListResult
	json.Unmarshal(resp.Result, &result)

	if len(result.Tools) != 26 {
		t.Errorf("tools count = %d, want 26", len(result.Tools))
		for _, tool := range result.Tools {
			t.Logf("  tool: %s", tool.Name)
		}
	}

	expected := map[string]bool{
		"list_surfaces": true, "get_tree": true, "get_component": true,
		"get_data_model": true, "get_layout": true, "get_style": true,
		"take_screenshot": true, "perform_action": true, "click": true, "fill": true,
		"toggle": true, "interact": true, "set_data_model": true,
		"wait_for": true, "send_message": true, "get_logs": true,
		"list_processes": true, "create_process": true, "stop_process": true,
		"send_to_process": true, "list_channels": true, "create_channel": true,
		"delete_channel": true, "publish": true, "subscribe": true,
		"unsubscribe": true,
	}
	for _, tool := range result.Tools {
		if !expected[tool.Name] {
			t.Errorf("unexpected tool: %s", tool.Name)
		}
		delete(expected, tool.Name)
	}
	for name := range expected {
		t.Errorf("missing tool: %s", name)
	}
}

func TestListSurfaces(t *testing.T) {
	srv := newServer()

	// No surfaces yet
	result := callTool(srv, "list_surfaces", `{}`)
	if result.IsError {
		t.Fatalf("error: %s", result.Content[0].Text)
	}
	if !strings.Contains(result.Content[0].Text, "[]") {
		t.Errorf("expected empty list, got: %s", result.Content[0].Text)
	}

	// Create a surface via send_message
	result = callTool(srv, "send_message", `{"message":{"type":"createSurface","surfaceId":"win1","title":"Test"}}`)
	if result.IsError {
		t.Fatalf("send_message error: %s", result.Content[0].Text)
	}

	result = callTool(srv, "list_surfaces", `{}`)
	if !strings.Contains(result.Content[0].Text, "win1") {
		t.Errorf("expected win1 in list, got: %s", result.Content[0].Text)
	}
}

func TestGetTree(t *testing.T) {
	srv := newServer()

	callTool(srv, "send_message", `{"message":{"type":"createSurface","surfaceId":"s1","title":"Test"}}`)
	callTool(srv, "send_message", `{"message":{"type":"updateComponents","surfaceId":"s1","components":[
		{"componentId":"root","type":"Column","children":["child1"]},
		{"componentId":"child1","type":"Text","parentId":"root","props":{"content":"Hello"}}
	]}}`)

	result := callTool(srv, "get_tree", `{"surface_id":"s1"}`)
	if result.IsError {
		t.Fatalf("error: %s", result.Content[0].Text)
	}
	if !strings.Contains(result.Content[0].Text, "root") {
		t.Errorf("expected 'root' in tree, got: %s", result.Content[0].Text)
	}
	if !strings.Contains(result.Content[0].Text, "child1") {
		t.Errorf("expected 'child1' in tree, got: %s", result.Content[0].Text)
	}
}

func TestGetTreeUnknownSurface(t *testing.T) {
	srv := newServer()

	result := callTool(srv, "get_tree", `{"surface_id":"nonexistent"}`)
	if !result.IsError {
		t.Error("expected error for unknown surface")
	}
}

func TestGetDataModel(t *testing.T) {
	srv := newServer()

	callTool(srv, "send_message", `{"message":{"type":"createSurface","surfaceId":"s1","title":"Test"}}`)
	callTool(srv, "send_message", `{"message":{"type":"updateDataModel","surfaceId":"s1","ops":[
		{"op":"add","path":"/name","value":"Alice"}
	]}}`)

	result := callTool(srv, "get_data_model", `{"surface_id":"s1","path":"/name"}`)
	if result.IsError {
		t.Fatalf("error: %s", result.Content[0].Text)
	}
	if !strings.Contains(result.Content[0].Text, "Alice") {
		t.Errorf("expected 'Alice', got: %s", result.Content[0].Text)
	}
}

func TestSetDataModel(t *testing.T) {
	srv := newServer()

	callTool(srv, "send_message", `{"message":{"type":"createSurface","surfaceId":"s1","title":"Test"}}`)
	result := callTool(srv, "set_data_model", `{"surface_id":"s1","path":"/color","value":"blue"}`)
	if result.IsError {
		t.Fatalf("error: %s", result.Content[0].Text)
	}

	result = callTool(srv, "get_data_model", `{"surface_id":"s1","path":"/color"}`)
	if !strings.Contains(result.Content[0].Text, "blue") {
		t.Errorf("expected 'blue', got: %s", result.Content[0].Text)
	}
}

func TestFill(t *testing.T) {
	srv := newServer()

	callTool(srv, "send_message", `{"message":{"type":"createSurface","surfaceId":"s1","title":"Test"}}`)
	callTool(srv, "send_message", `{"message":{"type":"updateComponents","surfaceId":"s1","components":[
		{"componentId":"field","type":"TextField","props":{"placeholder":"Name","dataBinding":"/name"}}
	]}}`)

	result := callTool(srv, "fill", `{"surface_id":"s1","component_id":"field","value":"Bob"}`)
	if result.IsError {
		t.Fatalf("fill error: %s", result.Content[0].Text)
	}

	result = callTool(srv, "get_data_model", `{"surface_id":"s1","path":"/name"}`)
	if !strings.Contains(result.Content[0].Text, "Bob") {
		t.Errorf("expected 'Bob' in data model, got: %s", result.Content[0].Text)
	}
}

func TestToggle(t *testing.T) {
	srv := newServer()

	callTool(srv, "send_message", `{"message":{"type":"createSurface","surfaceId":"s1","title":"Test"}}`)
	callTool(srv, "send_message", `{"message":{"type":"updateComponents","surfaceId":"s1","components":[
		{"componentId":"cb","type":"CheckBox","props":{"label":"Agree","dataBinding":"/agree"}}
	]}}`)

	result := callTool(srv, "toggle", `{"surface_id":"s1","component_id":"cb","checked":true}`)
	if result.IsError {
		t.Fatalf("toggle error: %s", result.Content[0].Text)
	}

	result = callTool(srv, "get_data_model", `{"surface_id":"s1","path":"/agree"}`)
	if !strings.Contains(result.Content[0].Text, "true") {
		t.Errorf("expected true in data model, got: %s", result.Content[0].Text)
	}
}

func TestSendMessage(t *testing.T) {
	srv := newServer()

	result := callTool(srv, "send_message", `{"message":{"type":"createSurface","surfaceId":"s1","title":"Hello"}}`)
	if result.IsError {
		t.Fatalf("error: %s", result.Content[0].Text)
	}

	ids := srv.sess.SurfaceIDs()
	found := false
	for _, id := range ids {
		if id == "s1" {
			found = true
		}
	}
	if !found {
		t.Error("surface s1 not created by send_message")
	}
}

func TestSendMessageInvalid(t *testing.T) {
	srv := newServer()

	result := callTool(srv, "send_message", `{"message":{"type":"unknownType"}}`)
	if !result.IsError {
		t.Error("expected error for unknown message type")
	}
}

func TestPing(t *testing.T) {
	srv := newServer()

	resp := srv.handle(&Request{Method: MethodPing})
	if resp.Error != nil {
		t.Fatalf("ping error: %s", resp.Error.Message)
	}
}

func TestUnknownMethod(t *testing.T) {
	srv := newServer()

	resp := srv.handle(&Request{Method: "nonexistent/method"})
	if resp.Error == nil {
		t.Fatal("expected error for unknown method")
	}
	if resp.Error.Code != MethodNotFound {
		t.Errorf("error code = %d, want %d", resp.Error.Code, MethodNotFound)
	}
}

func TestUnknownTool(t *testing.T) {
	srv := newServer()

	params, _ := json.Marshal(ToolCallParams{Name: "nonexistent_tool"})
	resp := srv.handle(&Request{
		Method: MethodToolsCall,
		Params: params,
	})
	if resp.Error == nil {
		t.Fatal("expected error for unknown tool")
	}
}

func TestGetComponent(t *testing.T) {
	srv := newServer()

	callTool(srv, "send_message", `{"message":{"type":"createSurface","surfaceId":"s1","title":"Test"}}`)
	callTool(srv, "send_message", `{"message":{"type":"updateComponents","surfaceId":"s1","components":[
		{"componentId":"heading","type":"Text","props":{"content":"Hello World"}}
	]}}`)

	result := callTool(srv, "get_component", `{"surface_id":"s1","component_id":"heading"}`)
	if result.IsError {
		t.Fatalf("error: %s", result.Content[0].Text)
	}
	if !strings.Contains(result.Content[0].Text, "Hello World") {
		t.Errorf("expected 'Hello World' in component, got: %s", result.Content[0].Text)
	}
}

func TestGetComponentNotFound(t *testing.T) {
	srv := newServer()

	callTool(srv, "send_message", `{"message":{"type":"createSurface","surfaceId":"s1","title":"Test"}}`)

	result := callTool(srv, "get_component", `{"surface_id":"s1","component_id":"nonexistent"}`)
	if !result.IsError {
		t.Error("expected error for nonexistent component")
	}
}

func TestEndToEndJSONRPC(t *testing.T) {
	srv := newServer()

	input := strings.Join([]string{
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1"}}}`,
		`{"jsonrpc":"2.0","method":"notifications/initialized"}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/list"}`,
		`{"jsonrpc":"2.0","id":3,"method":"ping"}`,
	}, "\n") + "\n"

	reader := strings.NewReader(input)
	var output bytes.Buffer
	transport := NewStdioTransport(reader, &output)

	err := srv.Run(t.Context(), transport)
	if err != nil {
		t.Fatalf("server error: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(output.String()), "\n")
	if len(lines) != 3 {
		t.Fatalf("expected 3 responses, got %d:\n%s", len(lines), output.String())
	}

	// Check initialize response
	var initResp Response
	json.Unmarshal([]byte(lines[0]), &initResp)
	if initResp.Error != nil {
		t.Errorf("initialize error: %s", initResp.Error.Message)
	}

	// Check tools/list response
	var toolsResp Response
	json.Unmarshal([]byte(lines[1]), &toolsResp)
	var toolsResult ToolsListResult
	json.Unmarshal(toolsResp.Result, &toolsResult)
	if len(toolsResult.Tools) != 26 {
		t.Errorf("tools count = %d, want 26", len(toolsResult.Tools))
	}

	// Check ping response
	var pingResp Response
	json.Unmarshal([]byte(lines[2]), &pingResp)
	if pingResp.Error != nil {
		t.Errorf("ping error: %s", pingResp.Error.Message)
	}
}

package transport

import (
	"context"
	"encoding/json"
	"jview/protocol"
	"testing"
	"time"

	anyllm "github.com/mozilla-ai/any-llm-go"
)

// mockProvider implements anyllm.Provider for testing.
type mockProvider struct {
	name       string
	responses  []anyllm.ChatCompletion
	callCount  int
	lastParams anyllm.CompletionParams
}

func (m *mockProvider) Name() string { return m.name }

func (m *mockProvider) Completion(ctx context.Context, params anyllm.CompletionParams) (*anyllm.ChatCompletion, error) {
	m.lastParams = params
	if m.callCount >= len(m.responses) {
		// Return a stop response
		return &anyllm.ChatCompletion{
			Choices: []anyllm.Choice{{
				FinishReason: anyllm.FinishReasonStop,
				Message:      anyllm.Message{Role: anyllm.RoleAssistant, Content: "done"},
			}},
		}, nil
	}
	resp := m.responses[m.callCount]
	m.callCount++
	return &resp, nil
}

func (m *mockProvider) CompletionStream(ctx context.Context, params anyllm.CompletionParams) (<-chan anyllm.ChatCompletionChunk, <-chan error) {
	chunks := make(chan anyllm.ChatCompletionChunk)
	errs := make(chan error, 1)
	close(chunks)
	errs <- nil
	return chunks, errs
}

func TestToolCallToMessage(t *testing.T) {
	tc := anyllm.ToolCall{
		ID:   "call_1",
		Type: "function",
		Function: anyllm.FunctionCall{
			Name:      "a2ui_createSurface",
			Arguments: `{"surfaceId":"s1","title":"Test"}`,
		},
	}

	msg, err := toolCallToMessage(tc)
	if err != nil {
		t.Fatalf("toolCallToMessage: %v", err)
	}
	if msg.Type != protocol.MsgCreateSurface {
		t.Errorf("expected createSurface, got %s", msg.Type)
	}
	cs, ok := msg.Body.(protocol.CreateSurface)
	if !ok {
		t.Fatalf("expected CreateSurface body, got %T", msg.Body)
	}
	if cs.SurfaceID != "s1" {
		t.Errorf("expected surfaceId s1, got %s", cs.SurfaceID)
	}
	if cs.Title != "Test" {
		t.Errorf("expected title Test, got %s", cs.Title)
	}
}

func TestToolCallToMessageUpdateComponents(t *testing.T) {
	args := map[string]interface{}{
		"surfaceId": "s1",
		"components": []interface{}{
			map[string]interface{}{
				"componentId": "txt1",
				"type":        "Text",
				"props": map[string]interface{}{
					"content": "Hello",
					"variant": "h1",
				},
			},
		},
	}
	argsJSON, _ := json.Marshal(args)

	tc := anyllm.ToolCall{
		ID:   "call_2",
		Type: "function",
		Function: anyllm.FunctionCall{
			Name:      "a2ui_updateComponents",
			Arguments: string(argsJSON),
		},
	}

	msg, err := toolCallToMessage(tc)
	if err != nil {
		t.Fatalf("toolCallToMessage: %v", err)
	}
	if msg.Type != protocol.MsgUpdateComponents {
		t.Errorf("expected updateComponents, got %s", msg.Type)
	}
	uc, ok := msg.Body.(protocol.UpdateComponents)
	if !ok {
		t.Fatalf("expected UpdateComponents body, got %T", msg.Body)
	}
	if len(uc.Components) != 1 {
		t.Fatalf("expected 1 component, got %d", len(uc.Components))
	}
	if uc.Components[0].ComponentID != "txt1" {
		t.Errorf("expected componentId txt1, got %s", uc.Components[0].ComponentID)
	}
}

func TestToolCallToMessageUnknownTool(t *testing.T) {
	tc := anyllm.ToolCall{
		Function: anyllm.FunctionCall{
			Name:      "unknown_tool",
			Arguments: "{}",
		},
	}
	_, err := toolCallToMessage(tc)
	if err == nil {
		t.Fatal("expected error for unknown tool")
	}
}

func TestLLMTransportToolMode(t *testing.T) {
	mock := &mockProvider{
		name: "mock",
		responses: []anyllm.ChatCompletion{
			{
				Choices: []anyllm.Choice{{
					FinishReason: anyllm.FinishReasonToolCalls,
					Message: anyllm.Message{
						Role: anyllm.RoleAssistant,
						ToolCalls: []anyllm.ToolCall{
							{
								ID:   "call_1",
								Type: "function",
								Function: anyllm.FunctionCall{
									Name:      "a2ui_createSurface",
									Arguments: `{"surfaceId":"s1","title":"Counter"}`,
								},
							},
						},
					},
				}},
			},
			{
				Choices: []anyllm.Choice{{
					FinishReason: anyllm.FinishReasonToolCalls,
					Message: anyllm.Message{
						Role: anyllm.RoleAssistant,
						ToolCalls: []anyllm.ToolCall{
							{
								ID:   "call_2",
								Type: "function",
								Function: anyllm.FunctionCall{
									Name:      "a2ui_updateComponents",
									Arguments: `{"surfaceId":"s1","components":[{"componentId":"txt1","type":"Text","props":{"content":"Count: 0","variant":"h1"}}]}`,
								},
							},
						},
					},
				}},
			},
			// stop response — end of initial turn
			{
				Choices: []anyllm.Choice{{
					FinishReason: anyllm.FinishReasonStop,
					Message:      anyllm.Message{Role: anyllm.RoleAssistant, Content: "UI created."},
				}},
			},
		},
	}

	tr := NewLLMTransport(LLMConfig{
		Provider: mock,
		Model:    "test-model",
		Prompt:   "Build a counter",
		Mode:     "tools",
	})
	tr.Start()

	// Collect messages
	var msgs []*protocol.Message
	timer := time.NewTimer(5 * time.Second)
	defer timer.Stop()

	// We expect 2 messages (createSurface + updateComponents), then the transport blocks waiting for an action
	for i := 0; i < 2; i++ {
		select {
		case msg, ok := <-tr.Messages():
			if !ok {
				t.Fatalf("messages closed early after %d messages", i)
			}
			msgs = append(msgs, msg)
		case <-timer.C:
			t.Fatalf("timeout waiting for message %d", i)
		}
	}

	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(msgs))
	}
	if msgs[0].Type != protocol.MsgCreateSurface {
		t.Errorf("msg[0]: expected createSurface, got %s", msgs[0].Type)
	}
	if msgs[1].Type != protocol.MsgUpdateComponents {
		t.Errorf("msg[1]: expected updateComponents, got %s", msgs[1].Type)
	}

	tr.Stop()
	// Drain remaining
	for range tr.Messages() {
	}
}

func TestLLMTransportSendActionTriggersNewTurn(t *testing.T) {
	turnCount := 0
	mock := &mockProvider{
		name: "mock",
		responses: []anyllm.ChatCompletion{
			// Turn 1: create surface
			{
				Choices: []anyllm.Choice{{
					FinishReason: anyllm.FinishReasonToolCalls,
					Message: anyllm.Message{
						Role: anyllm.RoleAssistant,
						ToolCalls: []anyllm.ToolCall{{
							ID:   "call_1",
							Type: "function",
							Function: anyllm.FunctionCall{
								Name:      "a2ui_createSurface",
								Arguments: `{"surfaceId":"s1","title":"Test"}`,
							},
						}},
					},
				}},
			},
			// Turn 1: stop
			{
				Choices: []anyllm.Choice{{
					FinishReason: anyllm.FinishReasonStop,
					Message:      anyllm.Message{Role: anyllm.RoleAssistant, Content: "Ready."},
				}},
			},
			// Turn 2 (after action): update components
			{
				Choices: []anyllm.Choice{{
					FinishReason: anyllm.FinishReasonToolCalls,
					Message: anyllm.Message{
						Role: anyllm.RoleAssistant,
						ToolCalls: []anyllm.ToolCall{{
							ID:   "call_3",
							Type: "function",
							Function: anyllm.FunctionCall{
								Name:      "a2ui_updateDataModel",
								Arguments: `{"surfaceId":"s1","ops":[{"op":"replace","path":"/count","value":1}]}`,
							},
						}},
					},
				}},
			},
			// Turn 2: stop
			{
				Choices: []anyllm.Choice{{
					FinishReason: anyllm.FinishReasonStop,
					Message:      anyllm.Message{Role: anyllm.RoleAssistant, Content: "Updated."},
				}},
			},
		},
	}

	// Override Completion to track turns
	origCompletion := mock.Completion
	_ = origCompletion
	mock2 := &countingMockProvider{mockProvider: mock, turnCount: &turnCount}

	tr := NewLLMTransport(LLMConfig{
		Provider: mock2,
		Model:    "test-model",
		Prompt:   "Build a counter",
		Mode:     "tools",
	})
	tr.Start()

	timer := time.NewTimer(5 * time.Second)
	defer timer.Stop()

	// Wait for first message (createSurface)
	select {
	case msg := <-tr.Messages():
		if msg.Type != protocol.MsgCreateSurface {
			t.Errorf("expected createSurface, got %s", msg.Type)
		}
	case <-timer.C:
		t.Fatal("timeout waiting for first message")
	}

	// Send an action to trigger turn 2
	tr.SendAction("s1", &protocol.Action{
		Type: "serverAction",
		Name: "increment",
	}, map[string]interface{}{"count": float64(0)})

	// Wait for the updateDataModel message from turn 2
	select {
	case msg := <-tr.Messages():
		if msg.Type != protocol.MsgUpdateDataModel {
			t.Errorf("expected updateDataModel, got %s", msg.Type)
		}
	case <-timer.C:
		t.Fatal("timeout waiting for second turn message")
	}

	tr.Stop()
	for range tr.Messages() {
	}
}

// countingMockProvider wraps mockProvider to count turns.
type countingMockProvider struct {
	mockProvider *mockProvider
	turnCount    *int
}

func (c *countingMockProvider) Name() string { return c.mockProvider.Name() }

func (c *countingMockProvider) Completion(ctx context.Context, params anyllm.CompletionParams) (*anyllm.ChatCompletion, error) {
	*c.turnCount++
	return c.mockProvider.Completion(ctx, params)
}

func (c *countingMockProvider) CompletionStream(ctx context.Context, params anyllm.CompletionParams) (<-chan anyllm.ChatCompletionChunk, <-chan error) {
	return c.mockProvider.CompletionStream(ctx, params)
}


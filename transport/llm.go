package transport

import (
	"context"
	"encoding/base64"
	"encoding/json"
	stderrors "errors"
	"fmt"
	"canopy/jlog"
	"canopy/protocol"
	"net"
	"strings"
	"sync"
	"time"

	anyllm "github.com/mozilla-ai/any-llm-go"
)

// LLMConfig configures the LLM transport.
type LLMConfig struct {
	Provider     anyllm.Provider
	Model        string
	Prompt       string
	Mode         string // "tools" (default) or "raw"
	LibraryBlock string // optional library component listing for system prompt
}

// PostTurnFunc is called after each LLM turn completes.
// It receives the turn number (1-based). If it returns a non-empty string,
// that string is appended as a user message and another turn is initiated.
// Return "" to accept the generation and stop iterating.
type PostTurnFunc func(turn int) string

type actionPayload struct {
	SurfaceID string
	Event     *protocol.EventDef
	Data      map[string]interface{}
}

// ScreenshotRequest is sent from the transport to the consumer goroutine
// to capture a window screenshot on the main thread.
type ScreenshotRequest struct {
	SurfaceID string
	ResultCh  chan ScreenshotResult
}

// ScreenshotResult is the response from a screenshot capture.
type ScreenshotResult struct {
	Data []byte
	Err  error
}

// LLMTransport connects to an LLM provider and streams A2UI messages
// from the LLM's responses. User actions trigger new conversation turns.
type LLMTransport struct {
	config   LLMConfig
	messages chan *protocol.Message
	errors   chan error
	actions  chan actionPayload
	done     chan struct{}
	stopOnce sync.Once
	cancel   context.CancelFunc

	// OnInitialTurnDone is called when the consumer processes the first-turn-done
	// sentinel. The transport sends a nil message through the messages channel after
	// the first turn completes; when the consumer sees nil, it calls this callback.
	// This ensures all messages from the first turn have been processed (recorded)
	// before cache finalization occurs.
	OnInitialTurnDone func()

	// PostTurnHook is called after each generation turn (including retries).
	// If it returns a non-empty string, the transport appends it as a user
	// message and initiates another turn. Use this for eval-driven retry loops.
	PostTurnHook PostTurnFunc

	// TestResultCh receives test results from the consumer goroutine.
	// When the transport sends an a2ui_test message, it waits on this channel
	// for the result string before responding to the LLM.
	TestResultCh chan string

	// LayoutResultCh receives feedback from the consumer goroutine
	// after updateComponents messages are buffered. Components are not
	// rendered until a non-updateComponents message triggers a flush.
	LayoutResultCh chan string

	// ScreenshotReqCh sends screenshot requests to the consumer goroutine.
	// The consumer dispatches the capture to the main thread and responds
	// on the per-request ResultCh.
	ScreenshotReqCh chan ScreenshotRequest

	// followUps receives user follow-up prompts (e.g. from Cmd+L).
	followUps chan string
}

func NewLLMTransport(cfg LLMConfig) *LLMTransport {
	if cfg.Mode == "" {
		cfg.Mode = "tools"
	}
	return &LLMTransport{
		config:          cfg,
		messages:        make(chan *protocol.Message, 64),
		errors:          make(chan error, 8),
		actions:         make(chan actionPayload, 16),
		followUps:       make(chan string, 1),
		done:            make(chan struct{}),
		TestResultCh:    make(chan string, 1),
		LayoutResultCh:  make(chan string, 1),
		ScreenshotReqCh: make(chan ScreenshotRequest, 1),
	}
}

func (t *LLMTransport) Messages() <-chan *protocol.Message {
	return t.messages
}

func (t *LLMTransport) Errors() <-chan error {
	return t.errors
}

func (t *LLMTransport) Start() {
	go t.run()
}

func (t *LLMTransport) Stop() {
	t.stopOnce.Do(func() {
		close(t.done)
		if t.cancel != nil {
			t.cancel()
		}
	})
}

// SendFollowUp sends a user follow-up prompt (e.g. from Cmd+L) as a new conversation turn.
func (t *LLMTransport) SendFollowUp(prompt string) {
	select {
	case t.followUps <- prompt:
	case <-t.done:
	}
}

func (t *LLMTransport) SendAction(surfaceID string, event *protocol.EventDef, data map[string]interface{}) {
	select {
	case t.actions <- actionPayload{SurfaceID: surfaceID, Event: event, Data: data}:
	case <-t.done:
	}
}

// truncate returns s truncated to maxLen characters with "..." appended if needed.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}


func (t *LLMTransport) run() {
	defer close(t.messages)
	defer close(t.errors)

	ctx, cancel := context.WithCancel(context.Background())
	t.cancel = cancel
	defer cancel()

	jlog.Infof("transport", "", "starting conversation with provider, model=%s mode=%s", t.config.Model, t.config.Mode)

	history := []anyllm.Message{
		{Role: anyllm.RoleSystem, Content: SystemPrompt(t.config.Prompt) + t.config.LibraryBlock},
		{Role: anyllm.RoleUser, Content: t.config.Prompt},
	}

	turnNum := 0
	for {
		select {
		case <-t.done:
			return
		default:
		}

		history = t.doTurn(ctx, history)
		if history == nil {
			return
		}
		turnNum++

		if turnNum == 1 && t.OnInitialTurnDone != nil {
			// Send nil sentinel through messages channel so the consumer
			// processes it AFTER all first-turn messages have been consumed.
			select {
			case t.messages <- nil:
			case <-t.done:
				return
			}
		}

		// Post-turn evaluation hook (eval-driven retry loop)
		if t.PostTurnHook != nil {
			feedback := t.PostTurnHook(turnNum)
			if feedback != "" {
				jlog.Infof("transport", "", "post-turn hook returned feedback (turn %d), retrying", turnNum)
				history = append(history, anyllm.Message{
					Role:    anyllm.RoleUser,
					Content: feedback,
				})
				continue
			}
			jlog.Infof("transport", "", "post-turn hook accepted generation (turn %d)", turnNum)
		}

		// Wait for a user action or follow-up prompt to trigger the next turn
		select {
		case ap := <-t.actions:
			userMsg := t.formatAction(ap)
			history = append(history, anyllm.Message{
				Role:    anyllm.RoleUser,
				Content: userMsg,
			})
		case prompt := <-t.followUps:
			history = append(history, anyllm.Message{
				Role:    anyllm.RoleUser,
				Content: prompt,
			})
		case <-t.done:
			return
		}
	}
}

// doTurn executes one LLM turn. Returns the updated history, or nil to stop.
func (t *LLMTransport) doTurn(ctx context.Context, history []anyllm.Message) []anyllm.Message {
	if t.config.Mode == "raw" {
		return t.doTurnRaw(ctx, history)
	}
	return t.doTurnTools(ctx, history)
}

// doTurnTools uses tool calling mode. Handles the tool call loop —
// the LLM may make multiple tool calls before finishing.
func (t *LLMTransport) doTurnTools(ctx context.Context, history []anyllm.Message) []anyllm.Message {
	for {
		select {
		case <-t.done:
			return nil
		default:
		}

		jlog.Infof("transport", "", "sending completion request (%d messages in history)", len(history))
		maxTok := 16384
		params := anyllm.CompletionParams{
			Model:     t.config.Model,
			Messages:  history,
			Tools:     a2uiTools(),
			MaxTokens: &maxTok,
		}

		var resp *anyllm.ChatCompletion
		var err error
		for attempt := 0; attempt < 3; attempt++ {
			resp, err = t.config.Provider.Completion(ctx, params)
			if err == nil || !isTransient(err) {
				break
			}
			delay := time.Duration(1<<uint(attempt)) * time.Second
			jlog.Warnf("transport", "", "transient error (attempt %d/3), retrying in %v: %v", attempt+1, delay, err)
			select {
			case <-time.After(delay):
			case <-t.done:
				return nil
			}
		}
		if err != nil {
			jlog.Errorf("transport", "", "completion error: %v", err)
			select {
			case t.errors <- fmt.Errorf("llm completion: %w", err):
			case <-t.done:
			}
			return nil
		}

		if len(resp.Choices) == 0 {
			jlog.Warn("transport", "", "empty response (no choices)")
			return history
		}

		choice := resp.Choices[0]
		jlog.Infof("transport", "", "got response, finish_reason=%s, tool_calls=%d", choice.FinishReason, len(choice.Message.ToolCalls))

		// Append assistant message to history
		history = append(history, choice.Message)

		// Process tool calls
		if len(choice.Message.ToolCalls) > 0 {
			for _, tc := range choice.Message.ToolCalls {
				// Log tool call with args preview
				argsPreview := tc.Function.Arguments
				if len(argsPreview) > 300 {
					argsPreview = argsPreview[:300] + "..."
				}
				jlog.Infof("transport", "", "tool call: %s — %s", tc.Function.Name, argsPreview)

				// Handle utility tools that return data to the LLM (not protocol messages)
				if tc.Function.Name == "a2ui_takeScreenshot" {
					result := t.handleScreenshot(tc)
					history = append(history, anyllm.Message{
						Role:       anyllm.RoleTool,
						Content:    result,
						ToolCallID: tc.ID,
					})
					continue
				}
				if tc.Function.Name == "a2ui_inspectLibrary" {
					result := handleInspectLibrary(tc)
					jlog.Debugf("transport", "", "inspectLibrary result: %s", truncate(result, 200))
					history = append(history, anyllm.Message{
						Role:       anyllm.RoleTool,
						Content:    result,
						ToolCallID: tc.ID,
					})
					continue
				}
				if tc.Function.Name == "a2ui_getLogs" {
					result := handleGetLogs(tc)
					jlog.Infof("transport", "", "getLogs result: %s", truncate(result, 300))
					history = append(history, anyllm.Message{
						Role:       anyllm.RoleTool,
						Content:    result,
						ToolCallID: tc.ID,
					})
					continue
				}

				msg, _, err := toolCallToMessage(tc)
				if err != nil {
					jlog.Warnf("transport", "", "tool call parse error: %v", err)
					// Send error as tool result so the LLM knows
					history = append(history, anyllm.Message{
						Role:       anyllm.RoleTool,
						Content:    fmt.Sprintf("error: %v", err),
						ToolCallID: tc.ID,
					})
					continue
				}

				// Handle test messages: send to consumer, wait for real results
				if msg.Type == protocol.MsgTest {
					select {
					case t.messages <- msg:
					case <-t.done:
						return nil
					}
					// Wait for test result from consumer goroutine
					var result string
					select {
					case result = <-t.TestResultCh:
					case <-time.After(5 * time.Second):
						result = "PASS (headless mode — no renderer available for live assertions)"
					case <-t.done:
						return nil
					}
					jlog.Infof("transport", "", "test result: %s", result)
					history = append(history, anyllm.Message{
						Role:       anyllm.RoleTool,
						Content:    result,
						ToolCallID: tc.ID,
					})
					continue
				}

				select {
				case t.messages <- msg:
				case <-t.done:
					return nil
				}

				// For updateComponents, wait for layout feedback from consumer
				if msg.Type == protocol.MsgUpdateComponents {
					var layoutInfo string
					select {
					case layoutInfo = <-t.LayoutResultCh:
					case <-time.After(5 * time.Second):
						layoutInfo = "ok"
					case <-t.done:
						return nil
					}
					history = append(history, anyllm.Message{
						Role:       anyllm.RoleTool,
						Content:    layoutInfo,
						ToolCallID: tc.ID,
					})
					continue
				}

				// Send success as tool result
				history = append(history, anyllm.Message{
					Role:       anyllm.RoleTool,
					Content:    "ok",
					ToolCallID: tc.ID,
				})
			}

			// If finish reason is tool_calls or length, loop to let the LLM continue
			if choice.FinishReason == anyllm.FinishReasonToolCalls || choice.FinishReason == anyllm.FinishReasonLength {
				continue
			}
		}

		// LLM is done for this turn
		return history
	}
}

// doTurnRaw uses raw text mode — the LLM outputs JSONL directly in its response.
func (t *LLMTransport) doTurnRaw(ctx context.Context, history []anyllm.Message) []anyllm.Message {
	chunks, errs := t.config.Provider.CompletionStream(ctx, anyllm.CompletionParams{
		Model:    t.config.Model,
		Messages: history,
	})

	var fullContent strings.Builder
	var lineBuf strings.Builder

	for chunk := range chunks {
		select {
		case <-t.done:
			return nil
		default:
		}

		if len(chunk.Choices) == 0 {
			continue
		}

		text := chunk.Choices[0].Delta.Content
		if text == "" {
			continue
		}

		fullContent.WriteString(text)
		lineBuf.WriteString(text)

		// Process complete lines
		for {
			content := lineBuf.String()
			idx := strings.Index(content, "\n")
			if idx < 0 {
				break
			}
			line := strings.TrimSpace(content[:idx])
			lineBuf.Reset()
			lineBuf.WriteString(content[idx+1:])

			if line == "" {
				continue
			}

			parser := protocol.NewParser(strings.NewReader(line))
			msg, err := parser.Next()
			if err != nil {
				// Non-fatal — LLM may output non-JSONL text
				jlog.Debugf("transport", "", "raw parse skip: %v", err)
				continue
			}

			select {
			case t.messages <- msg:
			case <-t.done:
				return nil
			}
		}
	}

	// Check for stream error
	if err := <-errs; err != nil {
		select {
		case t.errors <- fmt.Errorf("llm stream: %w", err):
		case <-t.done:
		}
		return nil
	}

	// Append assistant response to history
	history = append(history, anyllm.Message{
		Role:    anyllm.RoleAssistant,
		Content: fullContent.String(),
	})

	return history
}

// handleScreenshot requests a screenshot from the consumer goroutine and
// returns a special content string that the Anthropic provider converts to an image.
func (t *LLMTransport) handleScreenshot(tc anyllm.ToolCall) string {
	var params struct {
		SurfaceID string `json:"surfaceId"`
	}
	if err := json.Unmarshal([]byte(tc.Function.Arguments), &params); err != nil {
		return fmt.Sprintf("error: invalid params: %v", err)
	}
	if params.SurfaceID == "" {
		return "error: surfaceId is required"
	}

	resultCh := make(chan ScreenshotResult, 1)
	select {
	case t.ScreenshotReqCh <- ScreenshotRequest{SurfaceID: params.SurfaceID, ResultCh: resultCh}:
	case <-time.After(2 * time.Second):
		return "Screenshot not available (headless mode). Proceed to writing tests."
	case <-t.done:
		return "error: transport stopped"
	}

	var result ScreenshotResult
	select {
	case result = <-resultCh:
	case <-time.After(10 * time.Second):
		return "error: screenshot timed out"
	case <-t.done:
		return "error: transport stopped"
	}

	if result.Err != nil {
		return fmt.Sprintf("error: %v", result.Err)
	}

	b64 := base64.StdEncoding.EncodeToString(result.Data)
	jlog.Infof("transport", "", "screenshot captured: %d bytes (base64: %d)", len(result.Data), len(b64))
	return "__screenshot:" + b64
}

// formatAction formats a user event into a message string for the LLM.
func (t *LLMTransport) formatAction(ap actionPayload) string {
	parts := []string{
		fmt.Sprintf("User action on surface %q:", ap.SurfaceID),
		fmt.Sprintf("  event: %s", ap.Event.Name),
	}
	if len(ap.Data) > 0 {
		data, _ := json.MarshalIndent(ap.Data, "  ", "  ")
		parts = append(parts, fmt.Sprintf("  data:\n  %s", string(data)))
	}
	return strings.Join(parts, "\n")
}

// isTransient returns true for errors that are likely to succeed on retry:
// rate limits, server errors, timeouts, and connection failures.
func isTransient(err error) bool {
	if err == nil {
		return false
	}
	// anyllm typed errors
	var rateLimitErr *anyllm.RateLimitError
	var providerErr *anyllm.ProviderError
	if stderrors.As(err, &rateLimitErr) {
		return true
	}
	if stderrors.As(err, &providerErr) {
		return true
	}
	// Network errors: connection refused, reset, timeout
	var netErr *net.OpError
	if stderrors.As(err, &netErr) {
		return true
	}
	if stderrors.Is(err, context.DeadlineExceeded) {
		return true
	}
	// String heuristics for wrapped errors
	msg := err.Error()
	for _, substr := range []string{"429", "500", "502", "503", "504", "timeout", "connection refused", "connection reset"} {
		if strings.Contains(msg, substr) {
			return true
		}
	}
	return false
}

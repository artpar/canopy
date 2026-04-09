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
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	anyllm "github.com/mozilla-ai/any-llm-go"
)

// convLogger writes request/response payloads to a JSONL file for debugging.
type convLogger struct {
	f   *os.File
	enc *json.Encoder
}

func newConvLogger() *convLogger {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	dir := filepath.Join(home, ".canopy", "logs")
	if err := os.MkdirAll(dir, 0755); err != nil {
		jlog.Warnf("transport", "", "conv log: mkdir failed: %v", err)
		return nil
	}
	name := time.Now().Format("2006-01-02T15-04-05") + ".jsonl"
	f, err := os.Create(filepath.Join(dir, name))
	if err != nil {
		jlog.Warnf("transport", "", "conv log: create failed: %v", err)
		return nil
	}
	jlog.Infof("transport", "", "conv log: %s", f.Name())
	return &convLogger{f: f, enc: json.NewEncoder(f)}
}

func (cl *convLogger) Log(entry map[string]interface{}) {
	if cl == nil {
		return
	}
	entry["ts"] = time.Now().Format(time.RFC3339Nano)
	cl.enc.Encode(entry)
}

func (cl *convLogger) Close() {
	if cl == nil {
		return
	}
	cl.f.Close()
}

// LogTransition logs a state transition event.
func (cl *convLogger) LogTransition(t *Transition, turnCount int) {
	if cl == nil || t == nil {
		return
	}
	cl.Log(map[string]interface{}{
		"type":       "transition",
		"reason":     t.Reason,
		"detail":     t.Detail,
		"turn_count": turnCount,
	})
}

// LogTerminal logs a terminal event (loop exit).
func (cl *convLogger) LogTerminal(t *Terminal) {
	if cl == nil || t == nil {
		return
	}
	entry := map[string]interface{}{
		"type":       "terminal",
		"reason":     t.Reason,
		"turn_count": t.TurnCount,
	}
	if t.Err != nil {
		entry["error"] = t.Err.Error()
	}
	cl.Log(entry)
}

// LLMConfig configures the LLM transport.
type LLMConfig struct {
	Provider          anyllm.Provider
	Model             string
	Prompt            string
	Mode              string // "tools" (default) or "raw"
	LibraryBlock      string // optional library component listing for system prompt
	MaxTurns          int    // default 200, 0 = unlimited
	ContextWindowSize int    // default 200000 tokens
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

	// PreToolHook is called before each tool execution.
	// Return Block=true to prevent execution.
	PreToolHook PreToolHookFunc

	// PostToolHook is called after each tool execution.
	PostToolHook PostToolHookFunc

	// StopHook is called when the model stops (no tool_use blocks).
	// Can return blocking errors to force retry, or prevent continuation.
	StopHook StopHookFunc

	// SurfaceStateProvider returns a summary of current surface state
	// for injection as context between turns.
	SurfaceStateProvider func() string

	// attachmentProviders provides between-turn context injection.
	attachmentProviders []AttachmentProvider

	// followUps receives user follow-up prompts (e.g. from Cmd+L).
	followUps chan string

	// cl logs all request/response payloads to disk.
	cl *convLogger
}

func NewLLMTransport(cfg LLMConfig) *LLMTransport {
	if cfg.Mode == "" {
		cfg.Mode = "tools"
	}
	if cfg.MaxTurns == 0 {
		cfg.MaxTurns = 200
	}
	if cfg.ContextWindowSize == 0 {
		cfg.ContextWindowSize = 200000
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

// buildInitialMessages creates the initial conversation history.
func (t *LLMTransport) buildInitialMessages() []Message {
	return []Message{
		NewSystemMessage(BuildStaticPrefix(t.config.Prompt, t.config.LibraryBlock)),
		NewUserMessage(t.config.Prompt),
	}
}

// run is the main agentic loop, reimplemented as a state machine.
// Reference: query.ts:307-1728 queryLoop
//
// The loop has explicit transition tracking (7 continue reasons) and
// terminal conditions (12 return reasons). Each iteration calls
// loopIteration() which returns (next state, terminal).
func (t *LLMTransport) run() {
	defer close(t.messages)
	defer close(t.errors)

	t.cl = newConvLogger()
	defer t.cl.Close()

	ctx, cancel := context.WithCancel(context.Background())
	t.cancel = cancel
	defer cancel()

	jlog.Infof("transport", "", "starting conversation with provider, model=%s mode=%s", t.config.Model, t.config.Mode)

	// For raw mode, delegate to the legacy path (no state machine needed)
	if t.config.Mode == "raw" {
		t.runRaw(ctx)
		return
	}

	state := &LoopState{
		Messages:  t.buildInitialMessages(),
		TurnCount: 1,
	}

	for {
		// Check abort
		select {
		case <-t.done:
			t.cl.LogTerminal(&Terminal{Reason: TermAbortedStreaming})
			return
		default:
		}

		// Check max turns
		// Reference: query.ts:1704-1712
		if t.config.MaxTurns > 0 && state.TurnCount > t.config.MaxTurns {
			term := &Terminal{Reason: TermMaxTurns, TurnCount: state.TurnCount}
			t.cl.LogTerminal(term)
			jlog.Infof("transport", "", "max turns reached (%d), stopping", state.TurnCount)
			return
		}

		// Execute one iteration of the agentic loop
		next, term := t.loopIteration(ctx, state)

		if term != nil {
			t.cl.LogTerminal(term)

			// TermCompleted = normal turn completion (reference: query.ts:1357)
			// Don't exit — send nil sentinel, wait for user, loop again.
			if term.Reason == TermCompleted {
				// loopIteration already reset recovery counters.
				// Preserve accumulated state for next user-triggered turn.
				// Fall through to nil sentinel + user wait below.
			} else {
				// All other terminals are hard exits
				if term.Err != nil {
					select {
					case t.errors <- fmt.Errorf("llm %s: %w", term.Reason, term.Err):
					case <-t.done:
					}
				}
				return
			}
		} else {
			state = next
			t.cl.LogTransition(state.Transition, state.TurnCount)

			// Recovery transitions and tool continuations loop immediately (no user wait)
			if state.Transition != nil && state.Transition.Reason != TransNextTurn {
				continue
			}
		}

		// === Turn complete: send nil sentinel, wait for user ===

		// Send nil sentinel to signal turn completion.
		// The consumer uses this to flush pending components.
		select {
		case t.messages <- nil:
		case <-t.done:
			return
		}

		// Fire turn-done callback (one-shot per assignment)
		if t.OnInitialTurnDone != nil {
			t.OnInitialTurnDone()
			t.OnInitialTurnDone = nil
		}

		// Post-turn evaluation hook (eval-driven retry loop)
		// Reference: query/stopHooks.ts handleStopHooks
		if t.PostTurnHook != nil {
			feedback := t.PostTurnHook(state.TurnCount)
			if feedback != "" {
				jlog.Infof("transport", "", "post-turn hook returned feedback (turn %d), retrying", state.TurnCount)
				state.Messages = append(state.Messages, NewUserMessage(feedback))
				state.Transition = &Transition{Reason: TransStopHookBlocking, Detail: "feedback"}
				continue
			}
			jlog.Infof("transport", "", "post-turn hook accepted generation (turn %d)", state.TurnCount)
		}

		// Wait for a user action or follow-up prompt to trigger the next turn
		select {
		case ap := <-t.actions:
			userMsg := t.formatAction(ap)
			state.Messages = append(state.Messages, NewUserMessage(userMsg))
		case prompt := <-t.followUps:
			state.Messages = append(state.Messages, NewUserMessage(prompt))
		case <-t.done:
			return
		}
	}
}

// loopIteration executes one iteration of the agentic loop.
// Returns (nextState, nil) to continue or (nil, terminal) to stop.
// Reference: query.ts one iteration of while(true)
func (t *LLMTransport) loopIteration(ctx context.Context, state *LoopState) (*LoopState, *Terminal) {
	// === 1. PREPARE MESSAGES ===
	// Run 5 proactive context management layers (light to heavy).
	// Reference: query.ts:369-447
	preparedMessages := t.prepareMessagesForQuery(state)

	// Convert Message slice to anyllm.Message for the API
	messagesForQuery := normalizeForAPI(preparedMessages)

	// === Layer 5: Proactive auto-compaction ===
	// If approaching context window limit, compact before the API call.
	// Reference: query.ts:454-543 autoCompactIfNeeded
	if shouldAutoCompact(state.LastUsage, t.config.ContextWindowSize, 0) {
		tracking := state.AutoCompactTracking
		if tracking == nil || tracking.ConsecutiveFailures < MaxConsecutiveCompactFailures {
			compacted, compactErr := t.autoCompact(ctx, state)
			if compactErr != nil {
				jlog.Warnf("transport", "", "proactive auto-compact failed: %v", compactErr)
				if tracking == nil {
					tracking = &AutoCompactTracking{}
				}
				tracking.ConsecutiveFailures++
				state.AutoCompactTracking = tracking
			} else {
				state.Messages = compacted
				state.AutoCompactTracking = &AutoCompactTracking{Compacted: true}
				// Re-prepare after compaction
				preparedMessages = t.prepareMessagesForQuery(state)
				messagesForQuery = normalizeForAPI(preparedMessages)
				jlog.Infof("transport", "", "proactive auto-compact succeeded, %d messages after compaction", len(messagesForQuery))
			}
		}
	}

	// === Blocking limit check ===
	// Reference: query.ts:638-646 — if at hard limit and auto-compact exhausted, bail
	if state.LastUsage != nil {
		if state.LastUsage.PromptTokens >= t.config.ContextWindowSize-BlockingLimitBuffer {
			// Only block if auto-compact is exhausted (circuit breaker tripped)
			tracking := state.AutoCompactTracking
			if tracking != nil && tracking.ConsecutiveFailures >= MaxConsecutiveCompactFailures {
				jlog.Errorf("transport", "", "blocking limit reached (%d tokens, auto-compact exhausted)", state.LastUsage.PromptTokens)
				return nil, &Terminal{Reason: TermBlockingLimit, TurnCount: state.TurnCount}
			}
		}
	}

	// === 2. CALL MODEL ===
	maxTok := DefaultMaxTokens
	if state.MaxOutputTokensOverride != nil {
		maxTok = *state.MaxOutputTokensOverride
	}

	jlog.Infof("transport", "", "sending completion request (%d messages, maxTokens=%d, turn=%d)",
		len(messagesForQuery), maxTok, state.TurnCount)

	params := anyllm.CompletionParams{
		Model:     t.config.Model,
		Messages:  messagesForQuery,
		Tools:     a2uiTools(),
		MaxTokens: &maxTok,
	}

	t.cl.Log(map[string]interface{}{"type": "request", "model": params.Model, "messages": params.Messages})

	resp, err := t.callModelWithRetry(ctx, params)
	if err != nil {
		// Image/media validation error
		// Reference: query.ts:970-977
		if isImageError(err) {
			jlog.Errorf("transport", "", "image/media error: %v", err)
			return nil, &Terminal{Reason: TermImageError, Err: err, TurnCount: state.TurnCount}
		}

		// Context length error → reactive compaction recovery
		// Reference: query.ts:1085-1183
		if isContextLengthError(err) {
			if !state.HasAttemptedReactiveCompact {
				jlog.Warnf("transport", "", "context length exceeded, attempting reactive compaction")
				recovered, recoverErr := t.reactiveCompact(ctx, state)
				if recoverErr != nil {
					jlog.Errorf("transport", "", "reactive compaction failed: %v", recoverErr)
					return nil, &Terminal{Reason: TermPromptTooLong, Err: err, TurnCount: state.TurnCount}
				}
				return recovered, nil // loop continues with compacted state
			}
			return nil, &Terminal{Reason: TermPromptTooLong, Err: err, TurnCount: state.TurnCount}
		}

		jlog.Errorf("transport", "", "completion error: %v", err)
		return nil, &Terminal{Reason: TermModelError, Err: err, TurnCount: state.TurnCount}
	}

	if len(resp.Choices) == 0 {
		jlog.Warn("transport", "", "empty response (no choices)")
		state.Transition = &Transition{Reason: TransNextTurn}
		return state, nil
	}

	choice := resp.Choices[0]
	jlog.Infof("transport", "", "got response, finish_reason=%s, tool_calls=%d",
		choice.FinishReason, len(choice.Message.ToolCalls))

	t.cl.Log(map[string]interface{}{"type": "response", "finish_reason": choice.FinishReason, "message": choice.Message})

	// Capture usage for context management
	// Reference: query.ts captures usage for autocompact threshold checks
	if resp.Usage != nil {
		state.LastUsage = resp.Usage
	}

	// === 3. APPEND ASSISTANT MESSAGE ===
	state.Messages = append(state.Messages, NewAssistantMessage(choice.Message))

	// === Gap E: Abort check after model response, before processing ===
	// Reference: query.ts:1011-1052
	select {
	case <-t.done:
		for _, tc := range choice.Message.ToolCalls {
			state.Messages = append(state.Messages, NewToolResultMessage(tc.ID, "error: aborted", true))
		}
		state.Messages = append(state.Messages, NewInterruptionMessage())
		return nil, &Terminal{Reason: TermAbortedStreaming, TurnCount: state.TurnCount}
	default:
	}

	// === 4. CHECK FOR TOOL_USE ===
	needsFollowUp := len(choice.Message.ToolCalls) > 0

	if !needsFollowUp {
		// === NO TOOL_USE: STOP CONDITIONS ===
		// Reference: query.ts:1062-1357 (8 ordered checks)

		// 4a. Max output tokens recovery
		// Reference: query.ts:1188-1257
		if choice.FinishReason == anyllm.FinishReasonLength {
			// Phase 1: Escalate (one-shot)
			if state.MaxOutputTokensOverride == nil {
				escalated := EscalatedMaxTokens
				state.MaxOutputTokensOverride = &escalated
				state.Transition = &Transition{Reason: TransMaxOutputEscalate}
				jlog.Infof("transport", "", "max_output_tokens hit, escalating to %d", escalated)
				return state, nil
			}

			// Phase 2: Multi-turn recovery (up to 3)
			if state.MaxOutputRecoveryCount < MaxOutputTokensRecoveryLimit {
				state.MaxOutputRecoveryCount++
				state.Messages = append(state.Messages, NewMetaMessage(
					"Output token limit hit. Resume directly — no apology, no recap. "+
						"Pick up mid-thought if that is where the cut happened. "+
						"Break remaining work into smaller tool calls.",
				))
				state.Transition = &Transition{
					Reason: TransMaxOutputRecovery,
					Detail: fmt.Sprintf("attempt %d/%d", state.MaxOutputRecoveryCount, MaxOutputTokensRecoveryLimit),
				}
				jlog.Infof("transport", "", "max_output_tokens recovery attempt %d/%d",
					state.MaxOutputRecoveryCount, MaxOutputTokensRecoveryLimit)
				return state, nil
			}

			// Recovery exhausted
			jlog.Errorf("transport", "", "max_output_tokens recovery exhausted after %d attempts", MaxOutputTokensRecoveryLimit)
		}

		// 4b. Context collapse drain retry
		// Reference: query.ts:1085-1117
		// If approaching context limit and previous transition wasn't already a drain,
		// try collapsing more context to avoid hitting prompt-too-long on next call.
		if state.LastUsage != nil && state.Transition != nil && state.Transition.Reason != TransCollapseDrainRetry {
			threshold := t.config.ContextWindowSize - AutoCompactBufferTokens
			if state.LastUsage.PromptTokens >= threshold {
				collapsedMessages, collapsed := collapseOldContext(state.Messages)
				if collapsed > 0 {
					state.Messages = collapsedMessages
					state.Transition = &Transition{Reason: TransCollapseDrainRetry, Detail: fmt.Sprintf("collapsed %d rounds", collapsed)}
					jlog.Infof("transport", "", "context collapse drain: collapsed %d rounds, retrying", collapsed)
					return state, nil
				}
			}
		}

		// 4c. Stop hooks
		// Reference: query.ts:1267-1306
		if t.StopHook != nil {
			hookResult := t.StopHook(state.Messages, state.TurnCount, state.StopHookActive)
			if hookResult.PreventContinuation {
				return nil, &Terminal{Reason: TermStopHookPrevented, TurnCount: state.TurnCount}
			}
			if len(hookResult.BlockingErrors) > 0 {
				for _, errMsg := range hookResult.BlockingErrors {
					state.Messages = append(state.Messages, NewMetaMessage(errMsg))
				}
				state.StopHookActive = true
				state.Transition = &Transition{Reason: TransStopHookBlocking}
				return state, nil
			}
		}

		// 4d. Token budget continuation
		// Reference: query.ts:1308-1355 + query/tokenBudget.ts checkTokenBudget
		if state.LastUsage != nil && state.LastUsage.CompletionTokens > 0 {
			// Accumulate output tokens across continuations this turn
			state.GlobalTurnTokens += state.LastUsage.CompletionTokens

			// Initialize budget tracker on first check
			if state.BudgetTracker == nil {
				state.BudgetTracker = &BudgetTracker{}
			}

			budget := DefaultMaxTokens
			if state.MaxOutputTokensOverride != nil {
				budget = *state.MaxOutputTokensOverride
			}

			decision := checkTokenBudget(state.BudgetTracker, budget, state.GlobalTurnTokens)
			if decision.Action == BudgetContinue {
				state.Messages = append(state.Messages, NewMetaMessage(decision.NudgeMsg))
				state.Transition = &Transition{
					Reason: TransTokenBudgetContinuation,
					Detail: fmt.Sprintf("pct=%d%%, tokens=%d/%d, continuation=#%d",
						decision.Pct, decision.TurnTokens, decision.Budget, state.BudgetTracker.ContinuationCount),
				}
				jlog.Infof("transport", "", "token budget continuation #%d: %d%% (%d / %d), nudging",
					state.BudgetTracker.ContinuationCount, decision.Pct, decision.TurnTokens, decision.Budget)
				return state, nil
			}
		}

		// 4e. Normal completion
		// Reference: query.ts:1357 return { reason: 'completed' }
		// Reset recovery counters
		state.MaxOutputRecoveryCount = 0
		state.MaxOutputTokensOverride = nil
		state.HasAttemptedReactiveCompact = false
		state.BudgetTracker = nil
		state.GlobalTurnTokens = 0
		return nil, &Terminal{Reason: TermCompleted, TurnCount: state.TurnCount}
	}

	// === TOOL_USE DETECTED: EXECUTE TOOLS ===
	// Reference: query.ts:1360-1409
	// Dispatched through processToolCalls with batching (concurrent-safe tools
	// grouped, write-tools serial). See llm_tools_dispatch.go.
	results, execErr := t.processToolCalls(ctx, choice.Message.ToolCalls)
	if execErr != nil {
		state.Messages = append(state.Messages, NewInterruptionMessage())
		return nil, &Terminal{Reason: TermAbortedTools, Err: execErr, TurnCount: state.TurnCount}
	}

	// Abort check after tool execution
	// Reference: query.ts:1485-1516
	select {
	case <-t.done:
		state.Messages = append(state.Messages, NewInterruptionMessage())
		return nil, &Terminal{Reason: TermAbortedTools, TurnCount: state.TurnCount}
	default:
	}

	// Append tool results to history
	for _, result := range results {
		state.Messages = append(state.Messages, NewToolResultMessage(result.ToolCallID, result.Content, result.IsError))
	}

	// Inject attachments between turns
	// Reference: query.ts:1535-1671
	attachments := t.getAttachmentMessages(state)
	state.Messages = append(state.Messages, attachments...)

	// Gap G: Stop hook check after tool execution
	// Reference: query.ts:1519-1521
	if t.StopHook != nil {
		hookResult := t.StopHook(state.Messages, state.TurnCount, state.StopHookActive)
		if hookResult.PreventContinuation {
			return nil, &Terminal{Reason: TermHookStopped, TurnCount: state.TurnCount}
		}
	}

	// Reset recovery counters on successful tool turn
	// Reference: query.ts:1715-1728 state transition
	state.MaxOutputRecoveryCount = 0
	state.MaxOutputTokensOverride = nil
	state.HasAttemptedReactiveCompact = false
	state.BudgetTracker = nil
	state.GlobalTurnTokens = 0
	state.TurnCount++

	// Gap I: Post-compact turn tracking
	// Reference: query.ts:1523-1533
	if state.AutoCompactTracking != nil && state.AutoCompactTracking.Compacted {
		state.AutoCompactTracking.TurnCounter++
	}

	// Tool calls present → the LLM wants to continue with another round.
	// TransToolContinuation bypasses user-wait in run() loop.
	state.Transition = &Transition{Reason: TransToolContinuation}
	return state, nil
}

// callModelWithRetry calls the LLM with exponential backoff for transient errors.
// Reference: query.ts error handling with retry
func (t *LLMTransport) callModelWithRetry(ctx context.Context, params anyllm.CompletionParams) (*anyllm.ChatCompletion, error) {
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
			return nil, fmt.Errorf("transport stopped during retry")
		}
	}
	return resp, err
}

// runRaw handles the legacy raw text mode (no state machine).
func (t *LLMTransport) runRaw(ctx context.Context) {
	history := []anyllm.Message{
		{Role: anyllm.RoleSystem, Content: BuildStaticPrefix(t.config.Prompt, t.config.LibraryBlock)},
		{Role: anyllm.RoleUser, Content: t.config.Prompt},
	}

	for {
		select {
		case <-t.done:
			return
		default:
		}

		history = t.doTurnRaw(ctx, history)
		if history == nil {
			return
		}

		select {
		case t.messages <- nil:
		case <-t.done:
			return
		}

		if t.OnInitialTurnDone != nil {
			t.OnInitialTurnDone()
			t.OnInitialTurnDone = nil
		}

		select {
		case ap := <-t.actions:
			userMsg := t.formatAction(ap)
			history = append(history, anyllm.Message{Role: anyllm.RoleUser, Content: userMsg})
		case prompt := <-t.followUps:
			history = append(history, anyllm.Message{Role: anyllm.RoleUser, Content: prompt})
		case <-t.done:
			return
		}
	}
}

// doTurnRaw uses raw text mode — the LLM outputs JSONL directly in its response.
func (t *LLMTransport) doTurnRaw(ctx context.Context, history []anyllm.Message) []anyllm.Message {
	t.cl.Log(map[string]interface{}{"type": "request_raw", "model": t.config.Model, "messages": history})
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

	if err := <-errs; err != nil {
		select {
		case t.errors <- fmt.Errorf("llm stream: %w", err):
		case <-t.done:
		}
		return nil
	}

	content := fullContent.String()
	t.cl.Log(map[string]interface{}{"type": "response_raw", "content": content})
	history = append(history, anyllm.Message{
		Role:    anyllm.RoleAssistant,
		Content: content,
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

// isImageError checks if the error is related to image/media validation.
// Reference: query.ts:970-977 ImageSizeError, ImageResizeError
func isImageError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "image") && (strings.Contains(msg, "too large") || strings.Contains(msg, "invalid") || strings.Contains(msg, "size"))
}

// isContextLengthError checks if the error is a context length / prompt too long error.
// These need structured recovery (compaction), not blind retry.
func isContextLengthError(err error) bool {
	if err == nil {
		return false
	}
	var contextLenErr *anyllm.ContextLengthError
	if stderrors.As(err, &contextLenErr) {
		return true
	}
	// String fallback for wrapped errors
	msg := err.Error()
	return strings.Contains(msg, "prompt is too long") || strings.Contains(msg, "context length")
}

// isTransient returns true for errors that are likely to succeed on retry:
// rate limits, server errors, timeouts, and connection failures.
// Reference: query.ts error categorization
func isTransient(err error) bool {
	if err == nil {
		return false
	}
	// Context length errors need structured recovery, not blind retry
	var contextLenErr *anyllm.ContextLengthError
	if stderrors.As(err, &contextLenErr) {
		return false
	}
	// Auth/model errors are terminal
	var authErr *anyllm.AuthenticationError
	if stderrors.As(err, &authErr) {
		return false
	}
	var modelErr *anyllm.ModelNotFoundError
	if stderrors.As(err, &modelErr) {
		return false
	}
	var filterErr *anyllm.ContentFilterError
	if stderrors.As(err, &filterErr) {
		return false
	}

	// Transient: rate limits, provider errors
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

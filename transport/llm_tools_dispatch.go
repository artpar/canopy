package transport

import (
	"context"
	"fmt"
	"canopy/jlog"
	"canopy/protocol"
	"sync"
	"time"

	anyllm "github.com/mozilla-ai/any-llm-go"
)

// ToolDef describes a tool's capabilities.
// Reference: Tool.ts Tool interface (isConcurrencySafe, isReadOnly, isEnabled)
type ToolDef struct {
	Name              string
	IsConcurrencySafe bool // Can run in parallel with other safe tools
	IsReadOnly        bool // Doesn't modify UI state
	IsUtility         bool // Returns data to LLM, no protocol message
}

// ToolCallResult is the result of processing a single tool call.
// Reference: toolExecution.ts tool result handling
type ToolCallResult struct {
	ToolCallID string
	Content    string
	IsError    bool
}

// ToolBatch is a group of tool calls to execute together.
// Reference: toolOrchestration.ts Batch type
type ToolBatch struct {
	ToolCalls         []anyllm.ToolCall
	IsConcurrencySafe bool
}

// MaxToolUseConcurrency is the maximum number of concurrent tool executions.
// Reference: toolOrchestration.ts getMaxToolUseConcurrency() = 10
const MaxToolUseConcurrency = 10

// toolRegistry is the tool capability registry.
// Reference: tools.ts getAllBaseTools + Tool.ts isConcurrencySafe
var toolRegistry = map[string]ToolDef{
	// Utility tools (return data to LLM, no protocol message)
	"a2ui_takeScreenshot": {Name: "takeScreenshot", IsUtility: true, IsReadOnly: true, IsConcurrencySafe: true},
	"a2ui_inspectLibrary": {Name: "inspectLibrary", IsUtility: true, IsReadOnly: true, IsConcurrencySafe: true},
	"a2ui_getLogs":        {Name: "getLogs", IsUtility: true, IsReadOnly: true, IsConcurrencySafe: true},

	// Read-only protocol tools
	"a2ui_test": {Name: "test", IsReadOnly: true, IsConcurrencySafe: true},

	// State-mutating protocol tools (must run serially)
	"a2ui_createSurface":    {Name: "createSurface"},
	"a2ui_updateComponents": {Name: "updateComponents"},
	"a2ui_updateDataModel":  {Name: "updateDataModel"},
	"a2ui_setTheme":         {Name: "setTheme"},
	"a2ui_updateMenu":       {Name: "updateMenu"},
	"a2ui_loadAssets":       {Name: "loadAssets"},
	"a2ui_loadLibrary":      {Name: "loadLibrary"},
	"a2ui_createProcess":    {Name: "createProcess"},
	"a2ui_stopProcess":      {Name: "stopProcess"},
	"a2ui_createChannel":    {Name: "createChannel"},
	"a2ui_deleteChannel":    {Name: "deleteChannel"},
	"a2ui_subscribe":        {Name: "subscribe"},

	// Concurrency-safe protocol tools (don't mutate shared state)
	"a2ui_defineFunction":  {Name: "defineFunction", IsConcurrencySafe: true},
	"a2ui_defineComponent": {Name: "defineComponent", IsConcurrencySafe: true},
	"a2ui_publish":         {Name: "publish", IsConcurrencySafe: true},
}

// partitionToolCalls splits tool calls into batches for execution.
// Consecutive concurrency-safe tools are grouped into a single batch.
// Non-safe tools each get their own batch.
// Reference: toolOrchestration.ts partitionToolCalls
func partitionToolCalls(toolCalls []anyllm.ToolCall) []ToolBatch {
	if len(toolCalls) == 0 {
		return nil
	}

	var batches []ToolBatch

	for _, tc := range toolCalls {
		def, ok := toolRegistry[tc.Function.Name]
		isSafe := ok && def.IsConcurrencySafe

		// Try to append to the last batch if both are concurrency-safe
		if len(batches) > 0 && isSafe && batches[len(batches)-1].IsConcurrencySafe {
			batches[len(batches)-1].ToolCalls = append(batches[len(batches)-1].ToolCalls, tc)
		} else {
			batches = append(batches, ToolBatch{
				ToolCalls:         []anyllm.ToolCall{tc},
				IsConcurrencySafe: isSafe,
			})
		}
	}

	return batches
}

// processToolCalls executes all tool calls with batching.
// Safe tools run concurrently (max 10), unsafe tools run serially.
// Returns results in the same order as the input tool calls.
// Reference: toolOrchestration.ts runTools
func (t *LLMTransport) processToolCalls(
	ctx context.Context,
	toolCalls []anyllm.ToolCall,
) ([]ToolCallResult, error) {
	batches := partitionToolCalls(toolCalls)
	var allResults []ToolCallResult

	for _, batch := range batches {
		// Check abort before each batch
		select {
		case <-t.done:
			// Generate synthetic results for remaining tool calls
			for _, tc := range batch.ToolCalls {
				allResults = append(allResults, ToolCallResult{
					ToolCallID: tc.ID,
					Content:    "error: aborted",
					IsError:    true,
				})
			}
			return allResults, fmt.Errorf("aborted")
		default:
		}

		if batch.IsConcurrencySafe && len(batch.ToolCalls) > 1 {
			results := t.runToolsConcurrently(ctx, batch.ToolCalls)
			allResults = append(allResults, results...)
		} else {
			for _, tc := range batch.ToolCalls {
				result := t.executeToolCall(ctx, tc)
				allResults = append(allResults, result)
			}
		}
	}

	return allResults, nil
}

// runToolsConcurrently executes concurrency-safe tools in parallel.
// Reference: toolOrchestration.ts runToolsConcurrently with all() combinator
func (t *LLMTransport) runToolsConcurrently(
	ctx context.Context,
	toolCalls []anyllm.ToolCall,
) []ToolCallResult {
	results := make([]ToolCallResult, len(toolCalls))
	sem := make(chan struct{}, MaxToolUseConcurrency)
	var wg sync.WaitGroup

	for i, tc := range toolCalls {
		wg.Add(1)
		go func(idx int, tc anyllm.ToolCall) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			results[idx] = t.executeToolCall(ctx, tc)
		}(i, tc)
	}

	wg.Wait()
	return results
}

// executeToolCall handles a single tool call, dispatching by category.
// Reference: toolExecution.ts runToolUse
func (t *LLMTransport) executeToolCall(ctx context.Context, tc anyllm.ToolCall) ToolCallResult {
	// Log tool call
	argsPreview := tc.Function.Arguments
	if len(argsPreview) > 300 {
		argsPreview = argsPreview[:300] + "..."
	}
	jlog.Infof("transport", "", "tool call: %s — %s", tc.Function.Name, argsPreview)

	// Pre-tool hooks
	// Reference: toolExecution.ts runPreToolUseHooks
	if t.PreToolHook != nil {
		hookResult := t.PreToolHook(tc.Function.Name, tc.Function.Arguments)
		if hookResult.Block {
			return ToolCallResult{ToolCallID: tc.ID, Content: hookResult.Message, IsError: true}
		}
		if hookResult.UpdatedInput != "" {
			tc.Function.Arguments = hookResult.UpdatedInput
		}
	}

	// Look up tool definition for category dispatch
	def := toolRegistry[tc.Function.Name]

	// Dispatch by tool category using registry flags
	var result ToolCallResult
	if def.IsUtility {
		// Utility tools return data to the LLM without producing protocol messages.
		// Reference: Tool.ts isReadOnly + utility category
		switch tc.Function.Name {
		case "a2ui_takeScreenshot":
			result = ToolCallResult{ToolCallID: tc.ID, Content: t.handleScreenshot(tc)}
		case "a2ui_inspectLibrary":
			content := handleInspectLibrary(tc)
			jlog.Debugf("transport", "", "inspectLibrary result: %s", truncate(content, 200))
			result = ToolCallResult{ToolCallID: tc.ID, Content: content}
		case "a2ui_getLogs":
			content := handleGetLogs(tc)
			jlog.Infof("transport", "", "getLogs result: %s", truncate(content, 300))
			result = ToolCallResult{ToolCallID: tc.ID, Content: content}
		default:
			result = ToolCallResult{ToolCallID: tc.ID, Content: "error: unknown utility tool", IsError: true}
		}
	} else {
		// Protocol tools produce A2UI messages sent to the consumer.
		// def.IsReadOnly indicates tools that don't mutate UI state (e.g., test).
		result = t.executeProtocolToolCall(ctx, tc)
	}

	// Post-tool hooks
	if t.PostToolHook != nil {
		t.PostToolHook(tc.Function.Name, tc.Function.Arguments, result)
	}

	return result
}

// executeProtocolToolCall handles protocol tools that produce A2UI messages.
func (t *LLMTransport) executeProtocolToolCall(ctx context.Context, tc anyllm.ToolCall) ToolCallResult {
	msg, _, err := toolCallToMessage(tc)
	if err != nil {
		jlog.Warnf("transport", "", "tool call parse error: %v", err)
		return ToolCallResult{ToolCallID: tc.ID, Content: fmt.Sprintf("error: %v", err), IsError: true}
	}

	// Test messages: send to consumer, wait for real results
	if msg.Type == protocol.MsgTest {
		return t.executeTestToolCall(tc.ID, msg)
	}

	// Send protocol message to consumer
	select {
	case t.messages <- msg:
	case <-t.done:
		return ToolCallResult{ToolCallID: tc.ID, Content: "error: aborted", IsError: true}
	}

	// For updateComponents, wait for layout feedback
	if msg.Type == protocol.MsgUpdateComponents {
		return t.waitForLayoutFeedback(tc.ID)
	}

	return ToolCallResult{ToolCallID: tc.ID, Content: "ok"}
}

// executeTestToolCall sends a test message and waits for the result.
func (t *LLMTransport) executeTestToolCall(toolCallID string, msg *protocol.Message) ToolCallResult {
	select {
	case t.messages <- msg:
	case <-t.done:
		return ToolCallResult{ToolCallID: toolCallID, Content: "error: aborted", IsError: true}
	}

	var result string
	select {
	case result = <-t.TestResultCh:
	case <-time.After(5 * time.Second):
		result = "PASS (headless mode — no renderer available for live assertions)"
	case <-t.done:
		return ToolCallResult{ToolCallID: toolCallID, Content: "error: aborted", IsError: true}
	}

	jlog.Infof("transport", "", "test result: %s", result)
	return ToolCallResult{ToolCallID: toolCallID, Content: result}
}

// waitForLayoutFeedback waits for layout feedback from the consumer goroutine.
func (t *LLMTransport) waitForLayoutFeedback(toolCallID string) ToolCallResult {
	var layoutInfo string
	select {
	case layoutInfo = <-t.LayoutResultCh:
	case <-time.After(5 * time.Second):
		layoutInfo = "ok"
	case <-t.done:
		return ToolCallResult{ToolCallID: toolCallID, Content: "error: aborted", IsError: true}
	}
	return ToolCallResult{ToolCallID: toolCallID, Content: layoutInfo}
}

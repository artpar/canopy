package transport

// HookResult is the return from a pre/post tool hook execution.
// Reference: toolExecution.ts hook result handling
type HookResult struct {
	// Block prevents the tool from executing (PreToolUse only).
	Block bool

	// Message is the error message when Block is true, or additional context.
	Message string

	// UpdatedInput replaces the tool input (PreToolUse only).
	UpdatedInput string

	// PreventContinuation stops the loop (Stop hooks only).
	PreventContinuation bool
}

// StopHookResult is the combined result of all stop hooks.
// Reference: query/stopHooks.ts StopHookResult
type StopHookResult struct {
	// BlockingErrors are injected as user messages, forcing the model to retry.
	BlockingErrors []string

	// PreventContinuation stops the loop entirely.
	PreventContinuation bool
}

// PreToolHookFunc is called before each tool execution.
// It receives the tool name and arguments JSON string.
// Return Block=true to prevent execution.
// Reference: toolExecution.ts runPreToolUseHooks
type PreToolHookFunc func(toolName string, args string) HookResult

// PostToolHookFunc is called after each tool execution.
// It receives the tool name, arguments, and result.
// Reference: toolExecution.ts runPostToolUseHooks
type PostToolHookFunc func(toolName string, args string, result ToolCallResult) HookResult

// StopHookFunc is called when the model stops (no tool_use blocks).
// It receives the full message history, current turn count, and whether
// stop hooks already fired on a previous iteration (to prevent infinite loops).
// Reference: query/stopHooks.ts handleStopHooks
type StopHookFunc func(messages []Message, turnCount int, stopHookActive bool) StopHookResult

package transport

import (
	"fmt"

	anyllm "github.com/mozilla-ai/any-llm-go"
)

// Transition tracks why the loop continued instead of returning.
// Reference: query.ts State.transition
type Transition struct {
	Reason string
	Detail string // optional context (e.g., "attempt 2/3")
}

// Continue site reasons (7, matching reference query.ts)
const (
	TransNextTurn                = "next_turn"
	TransMaxOutputEscalate       = "max_output_tokens_escalate"
	TransMaxOutputRecovery       = "max_output_tokens_recovery"
	TransReactiveCompactRetry    = "reactive_compact_retry"
	TransCollapseDrainRetry      = "collapse_drain_retry"
	TransStopHookBlocking        = "stop_hook_blocking"
	TransTokenBudgetContinuation = "token_budget_continuation"
	TransToolContinuation        = "tool_continuation"
)

// Terminal tracks why the loop exited.
// Reference: query.ts Terminal
type Terminal struct {
	Reason    string
	Err       error
	TurnCount int
}

// Terminal reasons (matching reference query.ts)
const (
	TermCompleted         = "completed"
	TermAbortedStreaming   = "aborted_streaming"
	TermAbortedTools       = "aborted_tools"
	TermModelError         = "model_error"
	TermMaxTurns           = "max_turns"
	TermHookStopped        = "hook_stopped"
	TermPromptTooLong      = "prompt_too_long"
	TermBlockingLimit      = "blocking_limit"
	TermImageError         = "image_error"
	TermStopHookPrevented  = "stop_hook_prevented"
)

// LoopState is the single mutable state object carried across iterations.
// Reference: query.ts:204-217 State
type LoopState struct {
	// Messages is the full conversation history.
	Messages []Message

	// AutoCompactTracking tracks compaction state across turns.
	AutoCompactTracking *AutoCompactTracking

	// MaxOutputRecoveryCount tracks how many times we've retried on output truncation.
	// Limit: 3 (reference: MAX_OUTPUT_TOKENS_RECOVERY_LIMIT)
	MaxOutputRecoveryCount int

	// HasAttemptedReactiveCompact prevents infinite compact→retry loops.
	HasAttemptedReactiveCompact bool

	// MaxOutputTokensOverride escalates the output limit (nil = use default).
	// Phase 1: nil → 65536 on first length hit.
	MaxOutputTokensOverride *int

	// StopHookActive tracks whether stop hooks forced continuation on this iteration.
	StopHookActive bool

	// TurnCount is the 1-based turn counter.
	TurnCount int

	// Transition records why the previous iteration continued (nil on first iteration).
	Transition *Transition

	// LastUsage holds token usage from the most recent API response.
	// Used by context management to decide when to compact.
	LastUsage *anyllm.Usage

	// BudgetTracker tracks token budget continuation state.
	// Reference: query/tokenBudget.ts BudgetTracker
	BudgetTracker *BudgetTracker

	// GlobalTurnTokens is the cumulative output tokens across all continuations this turn.
	// Reference: query.ts getTurnOutputTokens()
	GlobalTurnTokens int
}

// BudgetTracker tracks token budget continuation state across nudge iterations.
// Reference: query/tokenBudget.ts BudgetTracker
type BudgetTracker struct {
	ContinuationCount    int
	LastDeltaTokens      int
	LastGlobalTurnTokens int
}

// BudgetCompletionThreshold is the fraction of output budget that triggers completion.
// Continue nudging while under this threshold.
// Reference: query/tokenBudget.ts COMPLETION_THRESHOLD = 0.9
const BudgetCompletionThreshold = 0.9

// BudgetDiminishingThreshold is the minimum delta (in tokens) between continuations
// to avoid being classified as diminishing returns.
// Reference: query/tokenBudget.ts DIMINISHING_THRESHOLD = 500
const BudgetDiminishingThreshold = 500

// AutoCompactTracking tracks compaction state across turns.
// Reference: services/compact/autoCompact.ts AutoCompactTrackingState
type AutoCompactTracking struct {
	Compacted           bool
	TurnID              string
	TurnCounter         int
	ConsecutiveFailures int
}

// MaxOutputTokensRecoveryLimit is the maximum number of resume-injection retries
// before giving up on max_output_tokens recovery.
// Reference: query.ts MAX_OUTPUT_TOKENS_RECOVERY_LIMIT = 3
const MaxOutputTokensRecoveryLimit = 3

// EscalatedMaxTokens is the escalated output token limit.
// Reference: utils/context.ts ESCALATED_MAX_TOKENS
const EscalatedMaxTokens = 65536

// DefaultMaxTokens is the default output token limit for A2UI generation.
const DefaultMaxTokens = 16384

// TokenBudgetAction is the result of checkTokenBudget.
type TokenBudgetAction int

const (
	// BudgetStop means the model should stop (budget met or diminishing returns).
	BudgetStop TokenBudgetAction = iota
	// BudgetContinue means the model should be nudged to continue.
	BudgetContinue
)

// TokenBudgetDecision is the result of a token budget check.
// Reference: query/tokenBudget.ts TokenBudgetDecision
type TokenBudgetDecision struct {
	Action      TokenBudgetAction
	NudgeMsg    string
	Pct         int
	TurnTokens  int
	Budget      int
}

// checkTokenBudget checks if the model should be nudged to continue working.
// Reference: query/tokenBudget.ts checkTokenBudget
func checkTokenBudget(tracker *BudgetTracker, budget int, globalTurnTokens int) TokenBudgetDecision {
	if budget <= 0 {
		return TokenBudgetDecision{Action: BudgetStop}
	}

	turnTokens := globalTurnTokens
	pct := 0
	if budget > 0 {
		pct = (turnTokens * 100) / budget
	}
	deltaSinceLastCheck := globalTurnTokens - tracker.LastGlobalTurnTokens

	isDiminishing := tracker.ContinuationCount >= 3 &&
		deltaSinceLastCheck < BudgetDiminishingThreshold &&
		tracker.LastDeltaTokens < BudgetDiminishingThreshold

	if !isDiminishing && turnTokens < int(float64(budget)*BudgetCompletionThreshold) {
		tracker.ContinuationCount++
		tracker.LastDeltaTokens = deltaSinceLastCheck
		tracker.LastGlobalTurnTokens = globalTurnTokens
		return TokenBudgetDecision{
			Action:     BudgetContinue,
			NudgeMsg:   getBudgetContinuationMessage(pct, turnTokens, budget),
			Pct:        pct,
			TurnTokens: turnTokens,
			Budget:     budget,
		}
	}

	return TokenBudgetDecision{Action: BudgetStop}
}

// getBudgetContinuationMessage builds the nudge message.
// Reference: utils/tokenBudget.ts getBudgetContinuationMessage
func getBudgetContinuationMessage(pct, turnTokens, budget int) string {
	return fmt.Sprintf("Stopped at %d%% of token target (%d / %d). Keep working — do not summarize.",
		pct, turnTokens, budget)
}

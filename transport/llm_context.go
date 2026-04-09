package transport

import (
	anyllm "github.com/mozilla-ai/any-llm-go"
)

// Context management constants matching the reference implementation.
// Reference: services/compact/autoCompact.ts
const (
	// AutoCompactBufferTokens is the buffer below context window for triggering auto-compact.
	// Reference: AUTOCOMPACT_BUFFER_TOKENS = 13_000
	AutoCompactBufferTokens = 13000

	// MaxConsecutiveCompactFailures is the circuit breaker limit.
	// Reference: MAX_CONSECUTIVE_AUTOCOMPACT_FAILURES = 3
	MaxConsecutiveCompactFailures = 3

	// ToolResultBudgetKeepRecent is how many recent tool results to keep unmodified.
	ToolResultBudgetKeepRecent = 8

	// ToolResultBudgetMaxChars is the max character count for old tool results.
	ToolResultBudgetMaxChars = 4000

	// MicrocompactThreshold is the total tool result count before microcompact triggers.
	MicrocompactThreshold = 30

	// MicrocompactKeepRecent is how many recent tool results microcompact preserves.
	MicrocompactKeepRecent = 15

	// SnipKeepMinMessages is the minimum message count to preserve during snip compaction.
	SnipKeepMinMessages = 20

	// BlockingLimitBuffer is the token buffer for the hard blocking limit check.
	// Reference: MANUAL_COMPACT_BUFFER_TOKENS = 3_000
	BlockingLimitBuffer = 3000
)

// prepareMessagesForQuery runs all proactive context management layers in order.
// Reference: query.ts:369-447 — 5 layers from light to heavy.
//
// The layers are:
//  1. Tool result budget (cap individual result sizes)
//  2. Snip compaction (remove oldest messages beyond minimum)
//  3. Microcompact (clear old tool result content with "[cleared]")
//  4. Context collapse (progressive summarization of old rounds)
//  5. Auto-compaction (full LLM-based summary, when approaching limit)
//
// Layer 6 (reactive compaction) is triggered on API error, not proactively.
func (t *LLMTransport) prepareMessagesForQuery(state *LoopState) []Message {
	messages := getMessagesAfterCompactBoundary(state.Messages)

	// Layer 1: Tool result budget — cap individual tool result sizes
	messages = budgetToolResults(messages, ToolResultBudgetKeepRecent, ToolResultBudgetMaxChars)

	// Layer 2: Snip compaction — remove oldest messages beyond minimum
	messages, _ = snipCompactIfNeeded(messages, SnipKeepMinMessages)

	// Layer 3: Microcompact — clear old tool result content
	messages = microcompact(messages, MicrocompactThreshold, MicrocompactKeepRecent)

	// Layer 4: Context collapse — progressive summarization of old conversation rounds
	messages, _ = collapseOldContext(messages)

	// Layer 5: Auto-compaction — full LLM-based summary (handled separately in loopIteration
	// since it requires an async LLM call)

	// Rebuild system prompt with dynamic context if SurfaceStateProvider is set.
	// Reference: utils/api.ts appendSystemContext — static prefix + dynamic suffix per turn.
	if t.SurfaceStateProvider != nil {
		dynamicSuffix := t.SurfaceStateProvider()
		if dynamicSuffix != "" {
			staticPrefix := BuildStaticPrefix(t.config.Prompt, t.config.LibraryBlock)
			fullPrompt := BuildFullSystemPrompt(SystemPromptConfig{
				StaticPrefix:  staticPrefix,
				DynamicSuffix: dynamicSuffix,
			})
			// Replace the system message with the rebuilt prompt
			for i, m := range messages {
				if m.Role == anyllm.RoleSystem {
					messages[i] = NewSystemMessage(fullPrompt)
					break
				}
			}
		}
	}

	return messages
}

// budgetToolResults caps old tool result sizes to keep context manageable.
// Keeps the most recent `keepRecent` tool results intact.
// Older tool results are truncated to `maxChars`, and screenshots are replaced entirely.
// Reference: utils/toolResultStorage.ts applyToolResultBudget
func budgetToolResults(messages []Message, keepRecent, maxChars int) []Message {
	// Count tool results from the end to identify which are "recent"
	toolResultIndices := make([]int, 0)
	for i, m := range messages {
		if m.Role == anyllm.RoleTool {
			toolResultIndices = append(toolResultIndices, i)
		}
	}

	if len(toolResultIndices) <= keepRecent {
		return messages // nothing to budget
	}

	// Indices of tool results that are NOT recent (eligible for budgeting)
	cutoff := len(toolResultIndices) - keepRecent
	budgetSet := make(map[int]bool, cutoff)
	for i := 0; i < cutoff; i++ {
		budgetSet[toolResultIndices[i]] = true
	}

	result := make([]Message, len(messages))
	copy(result, messages)

	for idx := range budgetSet {
		m := &result[idx]
		content := m.ContentString()

		// Screenshots: replace entirely (base64 can be >100KB)
		if isScreenshotContent(content) {
			m.Content = "[screenshot cleared from context]"
			continue
		}

		// Truncate long results
		if len(content) > maxChars {
			m.Content = content[:maxChars] + "... [truncated]"
		}
	}

	return result
}

// snipCompactIfNeeded removes oldest non-system messages when the count exceeds keepMin.
// Returns the snipped messages and an estimate of freed tokens.
// Reference: services/compact/snipCompact.ts snipCompactIfNeeded
func snipCompactIfNeeded(messages []Message, keepMin int) ([]Message, int) {
	if len(messages) <= keepMin {
		return messages, 0
	}

	// Separate system messages (always keep) from the rest
	var system []Message
	var rest []Message
	for _, m := range messages {
		if m.Role == anyllm.RoleSystem {
			system = append(system, m)
		} else {
			rest = append(rest, m)
		}
	}

	if len(rest) <= keepMin {
		return messages, 0
	}

	// Keep only the most recent keepMin non-system messages
	removed := rest[:len(rest)-keepMin]
	kept := rest[len(rest)-keepMin:]

	// Estimate freed tokens (rough: 4 chars per token)
	freedTokens := 0
	for _, m := range removed {
		freedTokens += len(m.ContentString()) / 4
	}

	result := make([]Message, 0, len(system)+len(kept))
	result = append(result, system...)
	result = append(result, kept...)
	return result, freedTokens
}

// microcompact replaces old tool result content with "[cleared]".
// Only triggers when total tool result count exceeds threshold.
// Keeps the most recent keepRecent tool results intact.
// Reference: services/compact/microCompact.ts microcompactMessages
func microcompact(messages []Message, threshold, keepRecent int) []Message {
	// Count tool results
	toolResultIndices := make([]int, 0)
	for i, m := range messages {
		if m.Role == anyllm.RoleTool {
			toolResultIndices = append(toolResultIndices, i)
		}
	}

	if len(toolResultIndices) <= threshold {
		return messages // below threshold, no action
	}

	// Clear old tool results beyond keepRecent
	cutoff := len(toolResultIndices) - keepRecent
	clearSet := make(map[int]bool, cutoff)
	for i := 0; i < cutoff; i++ {
		clearSet[toolResultIndices[i]] = true
	}

	result := make([]Message, len(messages))
	copy(result, messages)

	for idx := range clearSet {
		result[idx].Content = "[cleared]"
	}

	return result
}

// collapseOldContext progressively summarizes older conversation rounds.
// Groups messages into "rounds" (assistant + tool_results) and if there are
// more than 10 rounds, collapses the oldest rounds into a single summary message.
// Returns the collapsed messages and the number of rounds that were collapsed.
// Reference: services/contextCollapse/index.ts applyCollapsesIfNeeded
func collapseOldContext(messages []Message) ([]Message, int) {
	// Count assistant turns (each turn = assistant message + following tool results)
	turnCount := 0
	for _, m := range messages {
		if m.Role == anyllm.RoleAssistant {
			turnCount++
		}
	}

	// Only collapse if we have many turns
	if turnCount <= 10 {
		return messages, 0
	}

	// Keep the last 5 assistant turns intact
	keepTurns := 5
	turnsToKeep := turnCount - keepTurns
	if turnsToKeep <= 0 {
		return messages, 0
	}

	// Find the message index where the kept portion starts
	// (count backward through assistant messages)
	assistantCount := 0
	keepFromIdx := len(messages)
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == anyllm.RoleAssistant {
			assistantCount++
			if assistantCount >= keepTurns {
				keepFromIdx = i
				break
			}
		}
	}

	// Build collapsed summary from the old portion
	var summaryParts []string
	for _, m := range messages[:keepFromIdx] {
		if m.Role == anyllm.RoleSystem {
			continue // system messages handled separately
		}
		cs := m.ContentString()
		if len(cs) > 200 {
			cs = cs[:200] + "..."
		}
		if cs != "" && cs != "[cleared]" {
			summaryParts = append(summaryParts, cs)
		}
	}

	// Preserve system messages
	var result []Message
	for _, m := range messages {
		if m.Role == anyllm.RoleSystem {
			result = append(result, m)
			break
		}
	}

	// Add collapsed summary if there's content
	if len(summaryParts) > 0 {
		// Limit total summary size
		totalLen := 0
		var keptParts []string
		for _, p := range summaryParts {
			if totalLen+len(p) > 4000 {
				break
			}
			keptParts = append(keptParts, p)
			totalLen += len(p)
		}
		if len(keptParts) > 0 {
			collapsed := "[Earlier conversation collapsed]\n" + joinWithSeparator(keptParts, "\n---\n")
			result = append(result, NewMetaMessage(collapsed))
		}
	}

	// Append the kept recent portion
	result = append(result, messages[keepFromIdx:]...)
	return result, turnsToKeep
}

// shouldAutoCompact checks if token usage is approaching the context window limit.
// Reference: services/compact/autoCompact.ts shouldAutoCompact
func shouldAutoCompact(usage *anyllm.Usage, contextWindowSize, snipTokensFreed int) bool {
	if usage == nil {
		return false
	}
	effectiveTokens := usage.PromptTokens - snipTokensFreed
	threshold := contextWindowSize - AutoCompactBufferTokens
	return effectiveTokens >= threshold
}

// joinWithSeparator joins strings with a separator.
func joinWithSeparator(parts []string, sep string) string {
	if len(parts) == 0 {
		return ""
	}
	result := parts[0]
	for _, p := range parts[1:] {
		result += sep + p
	}
	return result
}

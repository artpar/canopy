package transport

import (
	"context"
	"fmt"
	"canopy/jlog"
	"regexp"
	"strings"

	anyllm "github.com/mozilla-ai/any-llm-go"
)

// compactionPrompt is the 9-section structured compaction prompt.
// Reference: services/compact/prompt.ts BASE_COMPACT_PROMPT
// The <analysis> block forces chain-of-thought before summarizing (discarded from output).
// The <summary> block contains the 9 sections that become the replacement history.
const compactionPrompt = `CRITICAL: Respond with TEXT ONLY. Do NOT call any tools.
- Do NOT use any tool calls. Tool calls will be REJECTED.
- You already have all the context you need in the conversation above.
- Tool calls will be REJECTED and will waste your only turn — you will fail the task.
- Your entire response must be plain text: an <analysis> block followed by a <summary> block.

You are summarizing a conversation between a user and an AI assistant that builds
native macOS UIs using the A2UI protocol. The summary will REPLACE the conversation
history, so it must contain all information needed to continue working.

First, write an <analysis> block with a chronological walkthrough of the conversation.
This is your drafting scratchpad — it will be discarded but improves summary quality.

Then write a <summary> block with these 9 sections:

1. Primary Request and Intent
   Describe in detail what the user asked for and their goals.

2. Key Technical Concepts
   List A2UI components, patterns, and techniques discussed.

3. Surfaces and Components
   List all surfaces created (IDs, titles) and their component trees
   with component IDs, types, and key prop values.

4. Data Model State
   Current data model values at each surface's JSON paths.

5. Errors and Fixes
   All errors encountered and how they were resolved.

6. All User Messages
   List ALL user messages that are NOT tool results.
   These are critical for understanding the user's feedback and changing intent.

7. Pending Tasks
   Tasks the user explicitly asked to work on that are not yet complete.

8. Current Work
   Precisely what was being worked on immediately before this summary,
   including specific component IDs and data model paths.

9. Optional Next Step
   Include DIRECT QUOTES from the most recent conversation showing exactly
   what task was being worked on and where you left off. This should be
   verbatim to ensure there's no drift in task interpretation.

CRITICAL: Respond with TEXT ONLY. Do NOT call any tools.`

// autoCompact performs full LLM-based conversation compaction.
// Reference: services/compact/compact.ts compactConversation
func (t *LLMTransport) autoCompact(ctx context.Context, state *LoopState) ([]Message, error) {
	jlog.Infof("transport", "", "starting auto-compaction (%d messages)", len(state.Messages))

	// Call LLM with compaction prompt (no tools, single turn, text-only)
	summary, err := t.callCompactionLLM(ctx, state.Messages)
	if err != nil {
		return nil, fmt.Errorf("compaction: %w", err)
	}

	// Format summary (strip <analysis>, extract <summary>)
	formatted := formatCompactSummary(summary)
	if formatted == "" {
		return nil, fmt.Errorf("compaction: empty summary after formatting")
	}

	// Build post-compact messages
	result := buildPostCompactMessages(state.Messages, formatted)

	jlog.Infof("transport", "", "compaction complete: %d messages → %d messages",
		len(state.Messages), len(result))

	t.cl.Log(map[string]interface{}{
		"type":          "compaction",
		"method":        "auto",
		"pre_messages":  len(state.Messages),
		"post_messages": len(result),
	})

	return result, nil
}

// callCompactionLLM calls the LLM to produce a summary.
// Reference: services/compact/compact.ts streamCompactSummary
// No tools allowed, single turn, aggressive no-tools preamble repeated.
func (t *LLMTransport) callCompactionLLM(ctx context.Context, messages []Message) (string, error) {
	// Build messages: existing conversation + compaction instruction
	compactMessages := make([]anyllm.Message, 0, len(messages)+1)
	for _, m := range messages {
		compactMessages = append(compactMessages, m.Message)
	}
	compactMessages = append(compactMessages, anyllm.Message{
		Role:    anyllm.RoleUser,
		Content: compactionPrompt,
	})

	maxTok := 16384
	resp, err := t.config.Provider.Completion(ctx, anyllm.CompletionParams{
		Model:     t.config.Model,
		Messages:  compactMessages,
		MaxTokens: &maxTok,
		// NO tools — text only
	})
	if err != nil {
		return "", fmt.Errorf("LLM call: %w", err)
	}

	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("empty response (no choices)")
	}

	return resp.Choices[0].Message.ContentString(), nil
}

// formatCompactSummary strips the <analysis> drafting scratchpad and extracts the <summary>.
// Reference: services/compact/prompt.ts formatCompactSummary
func formatCompactSummary(raw string) string {
	// Strip <analysis>...</analysis> block
	analysisRe := regexp.MustCompile(`(?s)<analysis>.*?</analysis>`)
	formatted := analysisRe.ReplaceAllString(raw, "")

	// Extract content from <summary>...</summary> tags
	summaryRe := regexp.MustCompile(`(?s)<summary>(.*?)</summary>`)
	matches := summaryRe.FindStringSubmatch(formatted)
	if len(matches) > 1 {
		formatted = strings.TrimSpace(matches[1])
	} else {
		// No <summary> tags — use the whole response (minus analysis)
		formatted = strings.TrimSpace(formatted)
	}

	return formatted
}

// buildPostCompactMessages constructs the post-compaction message array.
// Reference: services/compact/compact.ts buildPostCompactMessages
//
// Structure:
//  1. System message (unchanged — preserves prompt cache)
//  2. Compact boundary marker
//  3. Summary as user message with resume instruction
//  4. Last N recent messages (preserving recent context)
func buildPostCompactMessages(originalMessages []Message, summary string) []Message {
	result := make([]Message, 0)

	// 1. Preserve system messages (unchanged for cache stability)
	for _, m := range originalMessages {
		if m.Role == anyllm.RoleSystem {
			result = append(result, m)
		}
	}

	// 2. Compact boundary marker
	result = append(result, NewCompactBoundaryMessage())

	// 3. Summary as user message with resume instruction
	// Reference: services/compact/prompt.ts getCompactUserSummaryMessage
	summaryMsg := NewUserMessage(
		"This session is being continued from a previous conversation that ran out of context. " +
			"The summary below covers the earlier portion.\n\n" +
			summary + "\n\n" +
			"Continue from where you left off. Resume directly — do not acknowledge " +
			"the summary, do not recap, do not preface with \"I'll continue\" or similar. " +
			"Pick up the last task as if the break never happened.",
	)
	result = append(result, summaryMsg)

	// 4. Keep last 10 non-system messages from original (preserving recent context)
	// Reference: buildPostCompactMessages keeps messagesToKeep
	var nonSystem []Message
	for _, m := range originalMessages {
		if m.Role != anyllm.RoleSystem && !m.IsCompactBoundary {
			nonSystem = append(nonSystem, m)
		}
	}
	keepCount := 10
	if len(nonSystem) < keepCount {
		keepCount = len(nonSystem)
	}
	recent := nonSystem[len(nonSystem)-keepCount:]
	result = append(result, recent...)

	return result
}

// reactiveCompact is emergency recovery when the API returns context length error.
// Reference: query.ts:1119-1183 reactive compact retry
// Single-shot guard: state.HasAttemptedReactiveCompact prevents infinite retry.
func (t *LLMTransport) reactiveCompact(ctx context.Context, state *LoopState) (*LoopState, error) {
	jlog.Warnf("transport", "", "attempting reactive compaction (%d messages)", len(state.Messages))

	compacted, err := t.autoCompact(ctx, state)
	if err != nil {
		// Fallback: simple truncation
		jlog.Warnf("transport", "", "reactive compact failed (%v), falling back to simple truncation", err)
		compacted = simpleTruncate(state.Messages, 20)

		t.cl.Log(map[string]interface{}{
			"type":          "compaction",
			"method":        "simple_truncate",
			"pre_messages":  len(state.Messages),
			"post_messages": len(compacted),
		})
	}

	return &LoopState{
		Messages:                    compacted,
		TurnCount:                   state.TurnCount,
		MaxOutputRecoveryCount:      state.MaxOutputRecoveryCount,
		HasAttemptedReactiveCompact: true,
		AutoCompactTracking:         state.AutoCompactTracking,
		Transition:                  &Transition{Reason: TransReactiveCompactRetry},
	}, nil
}

// simpleTruncate keeps system messages + last N messages.
// Last resort when LLM compaction fails.
func simpleTruncate(messages []Message, keepLast int) []Message {
	var system []Message
	var rest []Message
	for _, m := range messages {
		if m.Role == anyllm.RoleSystem {
			system = append(system, m)
		} else {
			rest = append(rest, m)
		}
	}
	if len(rest) <= keepLast {
		return messages
	}
	truncated := rest[len(rest)-keepLast:]
	result := make([]Message, 0, len(system)+keepLast)
	result = append(result, system...)
	result = append(result, truncated...)
	return result
}

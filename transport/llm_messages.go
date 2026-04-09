package transport

import (
	"strings"
	"time"

	"github.com/google/uuid"
	anyllm "github.com/mozilla-ai/any-llm-go"
)

// Message wraps anyllm.Message with metadata matching the reference.
// Reference: types/message.ts UserMessage, AssistantMessage, etc.
type Message struct {
	anyllm.Message

	// UUID uniquely identifies this message.
	UUID string

	// Timestamp is when the message was created.
	Timestamp time.Time

	// IsMeta marks messages invisible to the user but visible to the model.
	// Used for recovery prompts, nudges, and system state injection.
	// Reference: query.ts isMeta pattern
	IsMeta bool

	// IsCompactBoundary marks this message as a compaction boundary.
	// Messages before a boundary are considered summarized.
	IsCompactBoundary bool
}

// NewUserMessage creates a user message.
func NewUserMessage(content string) Message {
	return Message{
		Message: anyllm.Message{
			Role:    anyllm.RoleUser,
			Content: content,
		},
		UUID:      uuid.NewString(),
		Timestamp: time.Now(),
	}
}

// NewMetaMessage creates a user message that is invisible to the user
// but visible to the model. Used for recovery prompts, nudges, etc.
// Reference: query.ts createUserMessage with isMeta: true
func NewMetaMessage(content string) Message {
	m := NewUserMessage(content)
	m.IsMeta = true
	return m
}

// NewSystemMessage creates a system message.
func NewSystemMessage(content string) Message {
	return Message{
		Message: anyllm.Message{
			Role:    anyllm.RoleSystem,
			Content: content,
		},
		UUID:      uuid.NewString(),
		Timestamp: time.Now(),
	}
}

// NewAssistantMessage creates an assistant message from an API response.
func NewAssistantMessage(msg anyllm.Message) Message {
	return Message{
		Message:   msg,
		UUID:      uuid.NewString(),
		Timestamp: time.Now(),
	}
}

// NewToolResultMessage creates a tool result message.
func NewToolResultMessage(toolCallID, content string, isError bool) Message {
	m := Message{
		Message: anyllm.Message{
			Role:       anyllm.RoleTool,
			Content:    content,
			ToolCallID: toolCallID,
		},
		UUID:      uuid.NewString(),
		Timestamp: time.Now(),
	}
	_ = isError // stored in content as "error: ..." prefix
	return m
}

// NewInterruptionMessage creates a user interruption message.
// Reference: utils/messages.ts createUserInterruptionMessage
func NewInterruptionMessage() Message {
	return NewMetaMessage("(user interrupted)")
}

// NewCompactBoundaryMessage creates a boundary marker for compaction.
func NewCompactBoundaryMessage() Message {
	m := NewSystemMessage("[compact boundary]")
	m.IsCompactBoundary = true
	return m
}

// normalizeForAPI ensures the message history is valid for the API.
// Reference: utils/messages.ts normalizeMessagesForAPI + claude.ts:1301 ensureToolResultPairing
//
// 1. Strip empty messages
// 2. Merge consecutive user messages (API rejects them)
// 3. Repair tool_use/tool_result pairing (ensureToolResultPairing)
// 4. Convert Message to anyllm.Message
func normalizeForAPI(messages []Message) []anyllm.Message {
	result := make([]anyllm.Message, 0, len(messages))

	for _, m := range messages {
		cs := m.ContentString()
		// Skip empty content messages (unless tool result or has tool calls)
		if cs == "" && len(m.ToolCalls) == 0 && m.Role != anyllm.RoleTool {
			continue
		}

		// Merge consecutive user messages
		if len(result) > 0 && result[len(result)-1].Role == anyllm.RoleUser && m.Role == anyllm.RoleUser {
			prevCS := result[len(result)-1].ContentString()
			result[len(result)-1].Content = prevCS + "\n\n" + cs
			continue
		}

		result = append(result, m.Message)
	}

	// Repair tool_use/tool_result pairing.
	// Reference: utils/messages.ts:5133-5415 ensureToolResultPairing, called at claude.ts:1301
	result = ensureToolResultPairing(result)

	return result
}

// SyntheticToolResultPlaceholder is the content used for synthetic error tool results.
// Reference: utils/messages.ts SYNTHETIC_TOOL_RESULT_PLACEHOLDER
const SyntheticToolResultPlaceholder = "error: tool execution interrupted"

// ensureToolResultPairing repairs tool_use/tool_result mismatches in the message array.
// For each assistant message with tool_use blocks, it checks the next message for
// matching tool_result blocks and:
//   - Generates synthetic error results for missing tool_results
//   - Strips orphaned tool_results (no matching tool_use in preceding assistant)
//   - Deduplicates tool_use IDs across assistant messages
//   - Deduplicates tool_result IDs
//
// Reference: utils/messages.ts:5133-5415 ensureToolResultPairing
func ensureToolResultPairing(messages []anyllm.Message) []anyllm.Message {
	result := make([]anyllm.Message, 0, len(messages))

	// Cross-message tool_use ID tracking for deduplication.
	// Reference: utils/messages.ts:5147 allSeenToolUseIds
	allSeenToolUseIDs := make(map[string]bool)

	for i := 0; i < len(messages); i++ {
		msg := messages[i]

		// Handle non-assistant messages
		if msg.Role != anyllm.RoleAssistant {
			// A tool message at the start or after a non-assistant has orphaned tool_results.
			// Reference: utils/messages.ts:5153-5200
			if msg.Role == anyllm.RoleTool && msg.ToolCallID != "" {
				prevIsAssistant := len(result) > 0 && result[len(result)-1].Role == anyllm.RoleAssistant
				if !prevIsAssistant {
					// Orphaned tool_result — skip it
					continue
				}
			}
			result = append(result, msg)
			continue
		}

		// === Assistant message: dedupe tool_use IDs ===
		// Reference: utils/messages.ts:5225-5243
		hasToolCalls := len(msg.ToolCalls) > 0
		if hasToolCalls {
			dedupedCalls := make([]anyllm.ToolCall, 0, len(msg.ToolCalls))
			for _, tc := range msg.ToolCalls {
				if allSeenToolUseIDs[tc.ID] {
					continue // duplicate tool_use ID across messages — strip
				}
				allSeenToolUseIDs[tc.ID] = true
				dedupedCalls = append(dedupedCalls, tc)
			}
			if len(dedupedCalls) != len(msg.ToolCalls) {
				msg.ToolCalls = dedupedCalls
			}
		}

		result = append(result, msg)

		if !hasToolCalls {
			continue
		}

		// Collect tool_use IDs from this assistant message
		toolUseIDSet := make(map[string]bool, len(msg.ToolCalls))
		for _, tc := range msg.ToolCalls {
			toolUseIDSet[tc.ID] = true
		}

		// Check following tool messages for matching tool_results
		// Reference: utils/messages.ts:5278-5316
		existingResultIDs := make(map[string]bool)
		seenResultIDs := make(map[string]bool)
		hasDuplicateResults := false

		// Look ahead at consecutive tool messages
		j := i + 1
		for j < len(messages) && messages[j].Role == anyllm.RoleTool {
			trID := messages[j].ToolCallID
			if trID != "" {
				if seenResultIDs[trID] {
					hasDuplicateResults = true
				}
				seenResultIDs[trID] = true
				existingResultIDs[trID] = true
			}
			j++
		}

		// Find missing IDs (tool_use without tool_result)
		var missingIDs []string
		for _, tc := range msg.ToolCalls {
			if !existingResultIDs[tc.ID] {
				missingIDs = append(missingIDs, tc.ID)
			}
		}

		// Find orphaned IDs (tool_result without tool_use)
		orphanedIDs := make(map[string]bool)
		for id := range existingResultIDs {
			if !toolUseIDSet[id] {
				orphanedIDs[id] = true
			}
		}

		// If everything matches, just append tool results and continue
		if len(missingIDs) == 0 && len(orphanedIDs) == 0 && !hasDuplicateResults {
			for i+1 < len(messages) && messages[i+1].Role == anyllm.RoleTool {
				i++
				result = append(result, messages[i])
			}
			continue
		}

		// === Repair needed ===

		// Generate synthetic error results for missing IDs
		// Reference: utils/messages.ts:5321-5326
		for _, id := range missingIDs {
			result = append(result, anyllm.Message{
				Role:       anyllm.RoleTool,
				Content:    SyntheticToolResultPlaceholder,
				ToolCallID: id,
			})
		}

		// Append existing tool results, stripping orphaned and duplicate ones
		// Reference: utils/messages.ts:5336-5352
		addedResultIDs := make(map[string]bool)
		for i+1 < len(messages) && messages[i+1].Role == anyllm.RoleTool {
			i++
			trID := messages[i].ToolCallID
			// Strip orphaned tool_results
			if orphanedIDs[trID] {
				continue
			}
			// Strip duplicate tool_results
			if addedResultIDs[trID] {
				continue
			}
			addedResultIDs[trID] = true
			result = append(result, messages[i])
		}
	}

	return result
}

// getMessagesAfterCompactBoundary returns messages after the last compaction boundary.
// Reference: utils/messages.ts getMessagesAfterCompactBoundary
func getMessagesAfterCompactBoundary(messages []Message) []Message {
	lastBoundary := -1
	for i, m := range messages {
		if m.IsCompactBoundary {
			lastBoundary = i
		}
	}
	if lastBoundary < 0 {
		return messages
	}
	return messages[lastBoundary:]
}

// WrapAsSystemReminder wraps content in <system-reminder> tags.
// Reference: utils/messages.ts system-reminder injection pattern
func WrapAsSystemReminder(content string) string {
	return "<system-reminder>\n" + content + "\n</system-reminder>"
}

// isScreenshotContent checks if content contains a screenshot.
func isScreenshotContent(content string) bool {
	return strings.HasPrefix(content, "__screenshot:")
}

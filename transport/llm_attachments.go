package transport

// AttachmentProvider is an interface for injecting context between turns.
// Implementations provide domain-specific context (surface state, task lists, etc.).
// Reference: utils/attachments.ts getAttachmentMessages
type AttachmentProvider interface {
	GetAttachments(state *LoopState) []Message
}

// getAttachmentMessages collects all between-turn context injections.
// Called after tool execution, before building next state.
// Reference: query.ts:1535-1671 attachment injection section
func (t *LLMTransport) getAttachmentMessages(state *LoopState) []Message {
	var attachments []Message

	// 1. Surface state snapshot (canopy-specific: replaces CLAUDE.md)
	// Injects current surface tree summary so the model knows what exists
	if t.SurfaceStateProvider != nil {
		stateMsg := t.SurfaceStateProvider()
		if stateMsg != "" {
			attachments = append(attachments, NewMetaMessage(
				WrapAsSystemReminder("Current surface state:\n"+stateMsg),
			))
		}
	}

	// 2. External attachment providers (extensible)
	for _, provider := range t.attachmentProviders {
		attachments = append(attachments, provider.GetAttachments(state)...)
	}

	return attachments
}

// RegisterAttachmentProvider adds an attachment provider for between-turn injection.
func (t *LLMTransport) RegisterAttachmentProvider(p AttachmentProvider) {
	t.attachmentProviders = append(t.attachmentProviders, p)
}

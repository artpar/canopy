package transport

// SystemPromptConfig holds the static/dynamic split for cache optimization.
// Reference: constants/prompts.ts static prefix + utils/api.ts dynamic suffix
//
// The static prefix is set once at session start and must NOT change between turns
// (cache stability). The dynamic suffix is rebuilt each turn with current state.
type SystemPromptConfig struct {
	// StaticPrefix is the cacheable system prompt prefix.
	// Contains: A2UI protocol rules, component catalog, expression language reference.
	StaticPrefix string

	// DynamicSuffix is rebuilt each turn with per-turn context.
	// Contains: active surfaces summary, data model snapshot, turn number.
	DynamicSuffix string
}

// BuildFullSystemPrompt constructs the full system prompt from static + dynamic parts.
// The boundary marker helps with prompt cache debugging.
// Reference: constants/prompts.ts SYSTEM_PROMPT_DYNAMIC_BOUNDARY
func BuildFullSystemPrompt(cfg SystemPromptConfig) string {
	if cfg.DynamicSuffix == "" {
		return cfg.StaticPrefix
	}
	return cfg.StaticPrefix + "\n\n--- DYNAMIC CONTEXT ---\n\n" + cfg.DynamicSuffix
}

// BuildStaticPrefix constructs the cacheable system prompt prefix.
// Must NOT change between turns (cache stability).
func BuildStaticPrefix(userPrompt, libraryBlock string) string {
	return SystemPrompt(userPrompt) + libraryBlock
}

# jview Roadmap

## Phase 1: MVP — COMPLETE

Protocol parsing, engine core, 7 component bridges, file transport, data binding, test infrastructure.

**Delivered:**
- JSONL parser with all 5 message types
- Engine: Session, Surface, Tree, DataModel, BindingTracker, Resolver
- Components: Text, Row, Column, Card, Button, TextField, CheckBox
- Two-way data binding with automatic propagation
- Callback lifecycle management (register/unregister on re-render)
- File transport with channel lifecycle guarantees
- CGo/ObjC bridge with ARC, target-action, associated objects
- Makefile: `make test` (headless, race-detected), `make verify` (screenshots), `make check` (gate)
- 74 tests across 3 packages with `-race` enabled

---

## Phase 2: Full Interactivity + Remaining Components — COMPLETE

Engine completeness and all remaining form/display components.

**Delivered:**
- FunctionCall evaluator with 14 built-in functions
- Validation engine with 5 rule types
- Template expansion for dynamic child lists
- 7 new component bridges: Divider, Icon, Image, Slider, ChoicePicker, DateTimeInput, List

---

## Phase 3: Media + Live Transport + Polish

Live agent connectivity and remaining A2UI components.

### Transport

| Task | Priority | Status | Description |
|------|----------|--------|-------------|
| LLM transport | critical | **done** | Bidirectional LLM transport via any-llm-go. Tool calling + raw JSONL modes. 7 providers (Anthropic, OpenAI, Gemini, Ollama, DeepSeek, Groq, Mistral). Default: Anthropic claude-haiku-4-5-20251001. |
| Action response pipeline | high | **done** | Button `dataRefs` resolved from DataModel, forwarded via `Transport.SendAction()`. LLM transport formats as user message → new turn. |
| Native e2e test framework | high | **done** | `jview test <file.jsonl>` runs inline test messages with real AppKit. 8 assertion types + event simulation. ObjC view queries for layout/style. LLM tool `a2ui_test` for LLM-generated tests. |
| SSE transport | medium | not started | `EventSource`-style HTTP streaming. Must pass `RunTransportContractTests`. |
| WebSocket transport | medium | not started | Bidirectional messaging. Must pass `RunTransportContractTests`. |
| stdin transport | low | not started | Read JSONL from stdin pipe. Useful for `agent | jview`. Must pass `RunTransportContractTests`. |

### Components

| Component | Priority | Status | Description |
|-----------|----------|--------|-------------|
| Tabs | high | **done** | NSTabView with tabLabels, activeTab data binding, tab selection callbacks. |
| Modal | high | **done** | NSPanel floating dialog with data-bound visible state, onDismiss callback. |
| Video | medium | **done** | AVPlayerView with src, autoplay, loop, controls, muted, onEnded callback. |
| AudioPlayer | low | **done** | AVPlayer compact control bar (play/pause, scrubber, time label) with src, autoplay, loop, onEnded. |

### Infrastructure

| Task | Priority | Description |
|------|----------|-------------|
| Theme support | low | `setTheme` message → `NSAppearance` switching (light/dark/system). |
| Scroll view for overflow | medium | Wrap root view in NSScrollView when content exceeds window. |

---

## Phase 4: Hardening + Process Model — COMPLETE

Reliability, process management, and always-on MCP.

**Delivered:**
- CGo memory cleanup: type-aware cleanup for Audio/Video/Modal (observer removal, player pause, panel close). CleanupAll dispatches RemoveView for all components.
- Error recovery: `logRecover` helper with stack traces. Panic recovery in Session.HandleMessage, per-component render, executeFunctionCall, and transport goroutines.
- Process model: `createProcess`, `stopProcess`, `sendToProcess` protocol messages. ProcessManager with TransportFactory. Three transport types: file, interval, LLM.
- Interval transport: sends fixed JSONL message on a timer with functionCall evaluation in op values.
- HandleUpdateDataModel now evaluates functionCalls in op values (matching executeUpdateDataModel).
- Process status written to `/processes/{id}/status` via HandleUpdateDataModel (triggers binding re-render).
- Always-on MCP: server starts on stdin/stdout in all modes (not just `jview mcp`). 19 tools including process management.
- LLM tools: `a2ui_createProcess`, `a2ui_stopProcess`.
- MCP tools: `list_processes`, `create_process`, `stop_process`, `send_to_process`, `get_logs`.

---

## Phase 5: Production Polish

Packaging and remaining transport modes.

| Task | Priority | Description |
|------|----------|-------------|
| Multi-surface window management | medium | Multiple windows from one session. Window positioning, focus management. |
| Incremental tree diff | medium | Only re-render components whose resolved props actually changed, not all affected components. |
| SSE transport | medium | `EventSource`-style HTTP streaming. Must pass `RunTransportContractTests`. |
| WebSocket transport | medium | Bidirectional messaging. Must pass `RunTransportContractTests`. |
| CLI flags | medium | `--title`, `--width`, `--height`, `--transport=sse\|ws\|file`, `--url`. |
| macOS .app bundle | low | Proper Info.plist, icon, code signing. Distribute as .dmg or via Homebrew. |

---

## Testing Strategy Per Phase

Each phase follows the same pattern:

1. **New component** → fixture in `testdata/`, E2E test in `engine/e2e_test.go`, integration test, screenshot verification
2. **New engine feature** → unit test for the feature, integration test with inline JSONL
3. **New transport** → must pass `RunTransportContractTests` from `transport/contract_test.go`
4. **All tests** run with `-race` enabled
5. **Gate** is always `make check` — headless tests + screenshot verification

---

## Decision Log

| Decision | Rationale |
|----------|-----------|
| Go + CGo + ObjC, not Swift | CGo can't call Swift directly. ObjC has stable C-compatible calling conventions. |
| Flat component map, not nested tree | A2UI protocol sends flat arrays with ID references. Matches wire format. |
| Topological sort per render batch | Components in the same `updateComponents` may reference each other as children. Must create leaves first. |
| Two-pass render (create, then set children) | Children must exist as native views before being added to containers. |
| Mock renderer + synchronous dispatcher | Enables headless testing without macOS display. All engine logic testable in CI. |
| Channel closure as transport contract | Prevents goroutine leaks. Enforced by `RunTransportContractTests`. |
| Callback unregister before re-register | Prevents stale closure capturing old binding paths. Old callback IDs cleaned up from registry. |
| `sync.Once` for transport Stop | Idempotent stop prevents double-close panic on `done` channel. |
| any-llm-go for LLM transport | Single SDK covers 9 providers with normalized API. Tool calling support. Channel-based streaming. Requires Go 1.25+. |
| Tool calling over raw JSONL | Tool calls give structured arguments → reliable parsing. Raw mode as fallback for models without tool support. |
| Non-streaming tool mode | Use `Completion()` not `CompletionStream()` for tool mode. Tool calls arrive atomically in non-streaming responses, avoiding partial argument assembly. |
| Default to Anthropic Haiku | Fast, cheap, good at tool calling. Sensible default for interactive UI generation. |
| Real AppKit for e2e tests | Tests query actual NSView frames, fonts, colors — not mock values. Synchronous MockDispatcher + real DarwinRenderer avoids dispatch_async deadlock on main thread. |
| Test messages inline in JSONL | Tests colocated with app definition. `jview` ignores them, `jview test` executes them. Single source of truth for app + tests. |

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

## Phase 2: Full Interactivity + Remaining Components

Engine completeness and all remaining form/display components.

### Engine Work

| Task | Priority | Status | Description |
|------|----------|--------|-------------|
| FunctionCall evaluator | high | not started | 14 built-in functions: `concat`, `format`, `toUpperCase`, `toLowerCase`, `trim`, `substring`, `length`, `add`, `subtract`, `multiply`, `divide`, `equals`, `greaterThan`, `not`. Placeholder exists in `engine/evaluator.go`. |
| Validation checks | high | not started | TextField validation rules: `required`, `minLength`, `maxLength`, `pattern`, `email`. Placeholder exists in `engine/validator.go`. Props field (`validations`) already parsed as `json.RawMessage`. |
| ChildList template expansion | high | not started | `forEach` + `templateId` → dynamic children from data model arrays. `ChildTemplate` struct already parsed. Engine needs to clone template components with item variable substitution. |

### New Components

| Component | Priority | Depends on | Notes |
|-----------|----------|------------|-------|
| Divider | medium | — | Simple `NSSeparator` or thin NSBox. Trivial bridge. |
| Slider | medium | — | NSSlider. Props already defined: `min`, `max`, `step`, `sliderValue`, `onSlide`. Needs data binding support. |
| Image | medium | — | NSImageView with async URL loading. Props: `src`, `alt`, `width`, `height`. Need to handle network fetch on background thread. |
| Icon | low | — | SF Symbols via `[NSImage imageWithSystemSymbolName:]`. Props: `name`, `size`. macOS 11+ only. |
| ChoicePicker | medium | — | NSPopUpButton (single) or NSMatrix/collection (multi). Props: `options`, `selected`, `mutuallyExclusive`, `onSelect`. |
| DateTimeInput | medium | — | NSDatePicker. Props: `enableDate`, `enableTime`, `dateValue`, `onDateChange`. |
| List | medium | template expansion | Scrollable NSScrollView + NSStackView with templated children. Depends on ChildList template expansion engine work. |

### Implementation Order

1. FunctionCall evaluator — unblocks dynamic UI logic
2. Validation checks — unblocks form completeness
3. Divider — quick win, useful in all layouts
4. Slider — extends interactive components
5. ChildList template expansion — unblocks List
6. Image — first async/network component
7. ChoicePicker — completes form components
8. DateTimeInput — completes form components
9. List — scrollable templated lists
10. Icon — low priority, macOS 11+ dependency

---

## Phase 3: Media + Live Transport + Polish

Live agent connectivity and remaining A2UI components.

### Transport

| Task | Priority | Depends on | Description |
|------|----------|------------|-------------|
| SSE transport | critical | — | `EventSource`-style HTTP streaming. Most common agent transport. Must pass `RunTransportContractTests`. |
| WebSocket transport | high | — | Bidirectional messaging. Must pass `RunTransportContractTests`. |
| Action response pipeline | high | SSE or WS | When user clicks a Button, send action back to the agent. Requires bidirectional channel or HTTP POST alongside SSE. |
| stdin transport | medium | — | Read JSONL from stdin pipe. Useful for `agent | jview`. Must pass `RunTransportContractTests`. |

### Components

| Component | Priority | Description |
|-----------|----------|-------------|
| Tabs | high | NSTabView with tab definitions. Props: `tabs` (array of `{id, label, children}`). |
| Modal | high | NSPanel or sheet presentation. Props: `visible`, `onDismiss`. Needs overlay management. |
| Video | medium | AVPlayerView. Props from A2UI spec TBD. |
| AudioPlayer | low | AVAudioPlayer with playback controls. |

### Infrastructure

| Task | Priority | Description |
|------|----------|-------------|
| Theme support | low | `setTheme` message → `NSAppearance` switching (light/dark/system). |
| Scroll view for overflow | medium | Wrap root view in NSScrollView when content exceeds window. |

---

## Phase 4: Production Hardening

Reliability, performance, packaging.

| Task | Priority | Description |
|------|----------|-------------|
| CGo memory cleanup | high | Audit all `unsafe.Pointer` usage. Ensure no ObjC objects leak. Add destructor tracking. |
| Error recovery | high | Graceful degradation when components fail to render. Surface-level error boundaries. |
| Multi-surface window management | medium | Multiple windows from one session. Window positioning, focus management. |
| Incremental tree diff | medium | Only re-render components whose resolved props actually changed, not all affected components. |
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

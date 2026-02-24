# Session Handoff

Last updated after Phase 3 LLM transport completion. This document gives a new session everything it needs to continue work on jview.

## What Is jview

A native macOS app that renders A2UI JSONL protocol as real AppKit widgets. Go engine processes messages, CGo bridge talks to Objective-C, native Cocoa views appear on screen. No webview. Connects to LLMs to generate native UIs in real-time — user interactions flow back as conversation turns.

## Current State

**Phase 1 complete.** Core rendering: Text, Row, Column, Card, Button, TextField, CheckBox with two-way data binding and callbacks.

**Phase 2 complete.** Full interactivity and all remaining form/display components:
- FunctionCall evaluator with 14 built-in functions
- Validation engine with 5 rule types
- Template expansion for dynamic child lists
- 7 new native component bridges: Divider, Icon, Image, Slider, ChoicePicker, DateTimeInput, List

**Phase 3 in progress.** LLM transport done:
- Bidirectional LLM transport via [any-llm-go](https://github.com/mozilla-ai/any-llm-go) v0.8.0
- 7 providers: Anthropic, OpenAI, Gemini, Ollama, DeepSeek, Groq, Mistral
- Default: Anthropic claude-haiku-4-5-20251001 (fast, cheap, good at tool calling)
- Two modes: "tools" (preferred, structured tool calls) and "raw" (JSONL in text stream)
- 5 A2UI tools map 1:1 to protocol message types, parsed through standard protocol.NewParser
- Action response: button `dataRefs` resolved from DataModel, sent back to LLM as user message → new turn
- Conversation loop runs for the lifetime of the process — each user action triggers a new LLM turn

**100+ tests pass** across protocol/, engine/, transport/ with race detection. 12 fixtures screenshot-verified.

## Repository Layout

```
main.go                       Entry point: locks OS thread, inits AppKit, starts transport
Makefile                       build / test / verify / check targets
spec.md                        A2UI protocol specification (as implemented)
plan.md                        Roadmap with phases 2-4

protocol/                      JSONL parsing, message types, dynamic values
  types.go                     Envelope, CreateSurface, DeleteSurface, UpdateDataModel, SetTheme
  component.go                 Component struct, Props (all component props in one struct)
  dynamic.go                   DynamicString, DynamicNumber, DynamicBoolean, DynamicStringList
  childlist.go                 ChildList (static array or template)
  action.go                    EventAction, Action, FunctionCall
  parse.go                     Parser (JSONL line reader)
  parse_test.go                15 parser tests including error paths

engine/                        Session routing, surface management, data model, bindings
  session.go                   Routes messages to surfaces by surfaceId
  surface.go                   Tree + DataModel + Bindings + Resolver + render dispatch
                               + template expansion + validation tracking + callback registration
  tree.go                      Flat component map, root detection, child ordering
  datamodel.go                 JSON Pointer get/set/delete with proper array shrinking
  binding.go                   BindingTracker: path -> component reverse index
  resolver.go                  Resolves DynamicValues against DataModel, registers bindings
                               Handles all 17 component types + function call evaluation
  evaluator.go                 FunctionCall evaluator: 14 functions, recursive arg resolution
  validator.go                 Validation engine: 5 rule types with custom messages
  evaluator_test.go            23 evaluator tests (all functions, nesting, paths, errors)
  validator_test.go            9 validator tests (all rules, custom messages, clearing)
  integration_test.go          Integration tests including slider, choicepicker, validation, templates
  e2e_test.go                  E2E tests: hello, contact_form, function_calls, list, layout
  *_test.go                    Unit tests for datamodel, binding, tree, resolver
  testhelper_test.go           goroutineLeakCheck, assertCreated, assertUpdated, newTestSession

renderer/                      Platform-agnostic interface
  renderer.go                  Renderer interface (CreateView, UpdateView, SetChildren, etc.)
  dispatch.go                  Dispatcher interface (RunOnMain)
  types.go                     ViewHandle, CallbackID, RenderNode, ResolvedProps, WindowSpec
                               OptionItem struct, ValidationErrors, date/choice/slider fields
  mock.go                      MockRenderer + MockDispatcher for headless testing

platform/darwin/               macOS CGo + ObjC implementation
  app.go/.h/.m                 NSApplication init and run loop
  renderer.go                  DarwinRenderer implementing Renderer interface (17 component types)
  dispatch.go/.h/.m            GCD-based main thread dispatcher
  registry.go                  CallbackRegistry (uint64 -> Go func)
  callback.go                  CGo callback bridge (GoCallbackInvoke)
  text.go/.h/.m                NSTextField (read-only label)
  stackview.go/.h/.m           NSStackView (Row + Column) with stretch alignment support
  card.go/.h/.m                NSBox with lowered content-hugging priority
  button.go/.h/.m              NSButton with target-action
  textfield.go/.h/.m           NSTextField (editable) with validation error display
  checkbox.go/.h/.m            NSButton (checkbox style)
  divider.go/.h/.m             NSBox separator
  icon.go/.h/.m                NSImageView with SF Symbols (systemSymbolName)
  image.go/.h/.m               NSImageView with async NSURLSession download
  slider.go/.h/.m              NSSlider with continuous target-action callbacks
  choicepicker.go/.h/.m        NSPopUpButton with option label/value pairs
  datetimeinput.go/.h/.m       NSDatePicker with ISO 8601 formatting
  list.go/.h/.m                Vertical NSStackView container (delegates to stackview)

transport/                     Message sources
  transport.go                 Transport interface (Messages, Errors, Start, Stop, SendAction)
  file.go                      FileTransport (reads JSONL from file, SendAction is no-op)
  llm.go                       LLMTransport (bidirectional LLM conversation loop)
  llm_tools.go                 5 A2UI tool definitions + toolCallToMessage converter + system prompt
  file_test.go                 5 channel lifecycle tests
  llm_test.go                  Mock provider tests: tool call parsing, transport lifecycle, action turns
  contract_test.go             RunTransportContractTests (reusable suite, includes SendActionDoesNotPanic)
  testhelper_test.go           goroutineLeakCheck, drain helpers

testdata/                      JSONL fixtures (12 total)
  hello.jsonl                  Card with heading + body text
  contact_form.jsonl           Form with data binding, preview card, checkbox, submit
  layout.jsonl                 Nested Row/Column with Cards and Button
  function_calls.jsonl         concat, toUpperCase, format with nested length
  divider.jsonl                Text above/below separator
  icon.jsonl                   Three SF Symbol icons in a row
  image.jsonl                  Image from URL with caption
  slider.jsonl                 Slider with data binding to display text
  choicepicker.jsonl           Dropdown with function call display
  datetimeinput.jsonl          Date picker with binding
  validation.jsonl             Form with required + minLength + email rules
  list.jsonl                   List with forEach template over 3 items
```

## Key Patterns

### Adding a New Component

1. Add type constant to `protocol/component.go`
2. Add props fields to `protocol.Props`
3. Add resolved fields to `renderer.ResolvedProps`
4. Add resolver case in `engine/resolver.go`
5. Create `platform/darwin/widget.go` + `.h` + `.m`
6. Add switch cases in `darwin.DarwinRenderer`: `CreateView`, `UpdateView`, `SetChildren`
7. Add callback registration in `engine/surface.go` if interactive
8. Add testdata fixture, integration test, `make check`

### Adding a New Transport

1. Implement `transport.Transport` interface (Messages, Errors, Start, Stop, SendAction)
2. Must pass `transport.RunTransportContractTests`
3. Both channels must close when done (prevents goroutine leaks)
4. Stop must be idempotent (use `sync.Once`)
5. `SendAction` can be no-op for read-only transports (file, stdin)

### CGo Rules

- Every `.go` file with `import "C"` needs `#cgo CFLAGS: -x objective-c -fobjc-arc`
- Each ObjC component = 3 files: `widget.go` + `widget.h` + `widget.m`
- `cgo.Handle` is integer — pass to `C.uintptr_t` directly, never `unsafe.Pointer`
- Use `objc_setAssociatedObject` to prevent target-action objects from being deallocated
- `callback.go` needs `#include <stdint.h>` for `C.uint64_t`

### Testing

- `make test` — headless, race-detected, no display needed
- `make verify` — builds binary, launches fixtures, captures screenshots
- `make check` — both (the gate, run before any commit)
- MockRenderer + MockDispatcher enable full engine testing without AppKit
- `goroutineLeakCheck(t)` — call at test start, defer the result

## Gotchas

1. **NSBox contentView** — never replace it. Add subviews to the existing contentView and pin with constraints.
2. **Root view bottom constraint** — use `constraintLessThanOrEqualToAnchor` so content sizes naturally from top, not `constraintEqualToAnchor`.
3. **Callback closures** — must unregister old callbacks before re-registering on re-render, otherwise the closure captures the stale binding path.
4. **Array deletion** — `deleteChild` uses `append(p[:idx], p[idx+1:]...)` to actually shrink the slice. The old code just nil'd the slot.
5. **Transport channel closure** — both `messages` and `errors` channels must close when the transport goroutine exits. Missing this causes goroutine leaks in consumers.
6. **Topological sort** — components in the same `updateComponents` batch may reference each other. Always create leaves before parents.
7. **Main thread** — all AppKit view operations must run on the main thread via `Dispatcher.RunOnMain()`. Go code runs on goroutines.
8. **NSStackView stretch alignment** — `NSLayoutAttributeWidth` does NOT stretch children to the stack's own width; it only equalizes sibling widths. For true stretch: use `NSLayoutAttributeLeading` alignment + explicitly pin each child's leading/trailing anchors to the stack in SetChildren. Store stretch flag via `objc_setAssociatedObject`.
9. **Template expansion deep copy** — shallow-copying components shares DynamicString pointers. Rewriting paths on one clone corrupts others. Always use `deepCopyComponent()` which copies all pointer fields.
10. **NSBox (Card) hugging** — NSBox has high content-hugging priority by default. Lowering it alone doesn't help without explicit width constraints from the parent stack.
11. **LLM tool call loop** — the LLM may return `finish_reason=tool_calls` multiple times in one turn (e.g. createSurface → updateDataModel → updateComponents). The transport loops until `finish_reason=stop`, then waits for a user action.
12. **Go 1.25 required** — any-llm-go v0.8.0 requires Go 1.25+. System Go may be older; use `~/go/bin/go1.25.0` for builds.

## What To Work On Next

See `plan.md` for the full roadmap. LLM transport is done. The immediate next priorities are:

1. **Tabs component** — tabbed container for multi-view layouts
2. **Modal component** — overlay dialogs
3. **SSE transport** — EventSource-style HTTP streaming for non-LLM agents
4. **WebSocket transport** — bidirectional messaging
5. **Video / AudioPlayer** — media components

## Commands

```bash
make build                           # Build binary to build/jview
make test                            # Headless tests with -race
make verify                          # Screenshot verification (12 fixtures)
make check                           # Full gate (test + verify)
build/jview testdata/hello.jsonl     # File mode (static fixture)
build/jview --prompt "Build a todo app"  # LLM mode (default: anthropic/haiku)
build/jview --llm openai --model gpt-4o --prompt "Build a calculator"
make verify-fixture F=testdata/hello.jsonl  # Single fixture screenshot
```

## Environment

- **Go 1.25.0** required (for any-llm-go dependency). Install: `go install golang.org/dl/go1.25.0@latest && go1.25.0 download`
- **ANTHROPIC_API_KEY** — set for default LLM mode. Other providers use their standard env vars (OPENAI_API_KEY, etc.)
- Use `~/go/bin/go1.25.0` if system Go is older than 1.25

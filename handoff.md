# Session Handoff

Last updated after the test infrastructure hardening session. This document gives a new session everything it needs to continue work on jview.

## What Is jview

A native macOS app that renders A2UI JSONL protocol as real AppKit widgets. Go engine processes messages, CGo bridge talks to Objective-C, native Cocoa views appear on screen. No webview.

## Current State

**Phase 1 complete.** The app renders Text, Row, Column, Card, Button, TextField, and CheckBox from JSONL files. Two-way data binding works (type in a field, bound labels update). Callbacks fire for buttons. Screenshots verify visual output.

**Test infrastructure hardened.** 74 tests, race detection on, goroutine leak checks, transport contract tests, error path coverage.

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
  tree.go                      Flat component map, root detection, child ordering
  datamodel.go                 JSON Pointer get/set/delete with proper array shrinking
  binding.go                   BindingTracker: path → component reverse index
  resolver.go                  Resolves DynamicValues against DataModel, registers bindings
  evaluator.go                 FunctionCall evaluator (Phase 2 placeholder)
  validator.go                 Validation checks (Phase 2 placeholder)
  *_test.go                    Unit, integration, E2E tests (54 tests total)
  testhelper_test.go           goroutineLeakCheck, assertCreated, assertUpdated, newTestSession

renderer/                      Platform-agnostic interface
  renderer.go                  Renderer interface (CreateView, UpdateView, SetChildren, etc.)
  dispatch.go                  Dispatcher interface (RunOnMain)
  types.go                     ViewHandle, CallbackID, RenderNode, ResolvedProps, WindowSpec
  mock.go                      MockRenderer + MockDispatcher for headless testing

platform/darwin/               macOS CGo + ObjC implementation
  app.go/.h/.m                 NSApplication init and run loop
  renderer.go                  DarwinRenderer implementing Renderer interface
  dispatch.go/.h/.m            GCD-based main thread dispatcher
  registry.go                  CallbackRegistry (uint64 → Go func)
  callback.go                  CGo callback bridge (GoCallbackInvoke)
  text.go/.h/.m                NSTextField (read-only label)
  stackview.go/.h/.m           NSStackView (Row + Column)
  card.go/.h/.m                NSBox
  button.go/.h/.m              NSButton with target-action
  textfield.go/.h/.m           NSTextField (editable)
  checkbox.go/.h/.m            NSButton (checkbox style)

transport/                     Message sources
  transport.go                 Transport interface
  file.go                      FileTransport (reads JSONL from file)
  file_test.go                 5 channel lifecycle tests
  contract_test.go             RunTransportContractTests (reusable suite)
  testhelper_test.go           goroutineLeakCheck, drain helpers

testdata/                      JSONL fixtures
  hello.jsonl                  Card with heading + body text
  contact_form.jsonl           Form with data binding, preview card, checkbox, submit
  layout.jsonl                 Nested Row/Column with Cards and Button
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

1. Implement `transport.Transport` interface (Messages, Errors, Start, Stop)
2. Must pass `transport.RunTransportContractTests`
3. Both channels must close when done (prevents goroutine leaks)
4. Stop must be idempotent (use `sync.Once`)

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

## What To Work On Next

See `plan.md` for the full roadmap. The immediate next priorities are:

1. **FunctionCall evaluator** (engine) — enables dynamic UI logic, unblocks many A2UI features
2. **Validation checks** (engine) — enables form validation feedback
3. **Divider component** — quick win, useful everywhere
4. **SSE transport** (critical for Phase 3) — connects to live AI agents

## Commands

```bash
make build                           # Build binary to build/jview
make test                            # Headless tests with -race
make verify                          # Screenshot verification
make check                           # Full gate
build/jview testdata/hello.jsonl     # Run interactively
make verify-fixture F=testdata/hello.jsonl  # Single fixture screenshot
```

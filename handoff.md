# Session Handoff

Last updated after Theme Switcher fix and MCP thread-safety (Phase 3 complete). This document gives a new session everything it needs to continue work on jview.

## What Is jview

A native macOS app that renders A2UI JSONL protocol as real AppKit widgets. Go engine processes messages, CGo bridge talks to Objective-C, native Cocoa views appear on screen. No webview. Connects to LLMs to generate native UIs in real-time — user interactions flow back as conversation turns.

## Current State

**Phase 1 complete.** Core rendering: Text, Row, Column, Card, Button, TextField, CheckBox with two-way data binding and callbacks.

**Phase 2 complete.** Full interactivity and all remaining form/display components:
- FunctionCall evaluator with 14 built-in functions
- Validation engine with 5 rule types
- Template expansion for dynamic child lists
- 7 new native component bridges: Divider, Icon, Image, Slider, ChoicePicker, DateTimeInput, List

**Phase 3 complete.** LLM transport, native testing, visual styling, FFI, and DX features done:
- Bidirectional LLM transport via [any-llm-go](https://github.com/mozilla-ai/any-llm-go) v0.8.0
- 7 providers: Anthropic, OpenAI, Gemini, Ollama, DeepSeek, Groq, Mistral
- Default: Anthropic claude-haiku-4-5-20251001 (fast, cheap, good at tool calling)
- Two modes: "tools" (preferred, structured tool calls) and "raw" (JSONL in text stream)
- 11 A2UI tools: createSurface, updateComponents, updateDataModel, deleteSurface, setTheme, test, loadAssets, loadLibrary, inspectLibrary, defineFunction, defineComponent
- Action response: button `dataRefs` resolved from DataModel, sent back to LLM as user message → new turn
- Conversation loop runs for the lifetime of the process — each user action triggers a new LLM turn
- Native e2e testing framework: `jview test <file.jsonl>` runs inline test messages with real AppKit rendering
- 8 assertion types (component, dataModel, children, notExists, count, action, layout, style) + event simulation
- ObjC view queries for layout (NSView frame) and style (font, color, opacity)
- Visual styling system: `StyleProps` on any component (backgroundColor, textColor, cornerRadius, width, height, fontSize, fontWeight, textAlign, opacity)
- Surface-level styling: window backgroundColor and configurable padding on createSurface
- `fillEqually` justify value for equal-width/height children in Row/Column
- Single `applyStyle()` function in platform layer — called after every CreateView/UpdateView, no per-component logic
- Button with custom backgroundColor → auto-switches to borderless mode so layer bg shows through
- Generic FFI via libffi: load any native .dylib and call C functions with arbitrary signatures (10 types, handle table for pointers, variadic support)
- Prompt caching for LLM-generated apps (SHA256 hash validation, atomic writes)
- 7 sample apps in `sample_apps/` including sysinfo (FFI demo with libcurl, libsqlite3, libz)
- DX abstractions: `defineFunction` (reusable parametric expressions), `defineComponent` (reusable component templates with ID rewriting + state scoping), `include` (file inclusion with circular detection), directory mode
- Tabs component: NSTabView with tabLabels, activeTab data binding, and tab selection callbacks
- Embedded MCP server: `jview mcp [file.jsonl]` with 14 tools (query, interact, screenshot, send_message) on stdin/stdout JSON-RPC 2.0
- flexGrow style property: children expand to fill available space in Row/Column via manual Auto Layout constraint chains (bypasses NSStackView distribution)
- forEach action rewriting: onClick/onChange/etc. actions in templates get data model paths rewritten per iteration
- forEach clone ID namespacing: IDs prefixed by parent List ID to avoid collisions across multiple lists
- Modal component: NSPanel floating dialog with data-bound visible state, onDismiss callback, and children layout
- Video component: AVPlayerView with src (data-bound), autoplay, loop, controls, muted, onEnded callback, URL change detection
- AudioPlayer component: AVPlayer with compact control bar (play/pause, scrubber, time label), src, autoplay, loop, onEnded callback
- Theme (NSAppearance): `setTheme` message + `setTheme` built-in functionCall action for client-side theme switching (light/dark/system)
- Scroll View: List component wraps NSStackView in NSScrollView for overflow handling
- MCP thread-safety: interaction tools (click/fill/toggle/interact) wrapped in dispatchSync to run on main thread with render flush
- MCP OnAction wiring: no-op action handler prevents nil panic when buttons fire events in MCP mode

**300+ tests pass** across protocol/, engine/, transport/ with race detection. 31 fixtures screenshot-verified.

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
  parse_test.go                21 parser tests including style, error paths, defineFunction/defineComponent/include

engine/                        Session routing, surface management, data model, bindings, FFI
  session.go                   Routes messages to surfaces by surfaceId, handles loadLibrary,
                               defineFunction, defineComponent
  surface.go                   Tree + DataModel + Bindings + Resolver + render dispatch
                               + template expansion + component instance expansion
                               + validation tracking + callback registration
  substitution.go              Shared JSON tree walkers: substituteParams, rewriteComponentIDs,
                               rewriteScopedPaths, deepCopyJSON
  tree.go                      Flat component map, root detection, child ordering
  datamodel.go                 JSON Pointer get/set/delete with proper array shrinking
  binding.go                   BindingTracker: path -> component reverse index
  resolver.go                  Resolves DynamicValues against DataModel, registers bindings
                               Handles all 18 component types + function call evaluation
  evaluator.go                 FunctionCall evaluator: 14 built-in + user-defined + FFI fallthrough, recursive arg resolution
  validator.go                 Validation engine: 5 rule types with custom messages
  ffilib.go                    Generic FFI via libffi: dlopen, ffi_prep_cif, ffi_call, handle table
  ffilib_config.go             FFI config loading (JSON file with library/function declarations)
  ffilib_test.go               FFI unit tests (typed calls, handle table, error cases, session integration)
  ffi_e2e_test.go              FFI e2e tests with real system libraries (libcurl, libsqlite3, libz)
  evaluator_test.go            23 evaluator tests (all functions, nesting, paths, errors)
  validator_test.go            9 validator tests (all rules, custom messages, clearing)
  substitution_test.go         8 tests for substituteParams, rewriteComponentIDs, rewriteScopedPaths
  integration_test.go          Integration tests including slider, choicepicker, validation, templates,
                               defineFunction, defineComponent, state scoping
  testrunner.go                Native e2e test runner (real AppKit assertions, 8 assert types)
  testrunner_test.go           Test runner tests (all assertion types, edge cases, simulation, integration)
  e2e_test.go                  E2E tests: hello, contact_form, function_calls, list, layout, calculator,
                               custom_functions, component_defs, includes, calculator_v2, scoped_components,
                               modal, video, audio
  *_test.go                    Unit tests for datamodel, binding, tree, resolver
  testhelper_test.go           goroutineLeakCheck, assertCreated, assertUpdated, newTestSession

renderer/                      Platform-agnostic interface
  renderer.go                  Renderer interface (CreateView, UpdateView, SetChildren, etc.)
  dispatch.go                  Dispatcher interface (RunOnMain)
  types.go                     ViewHandle, CallbackID, RenderNode, ResolvedProps, WindowSpec
                               OptionItem struct, ValidationErrors, LayoutInfo, StyleInfo
  mock.go                      MockRenderer + MockDispatcher for headless testing

platform/darwin/               macOS CGo + ObjC implementation
  app.go/.h/.m                 NSApplication init/run loop + AppStop/AppRunUntilIdle/ForceLayout
  viewquery.go/.h/.m           ObjC view frame/style queries (JVGetViewFrame, JVGetViewStyle)
  renderer.go                  DarwinRenderer implementing Renderer interface (18 component types + InvokeCallback + QueryLayout/Style)
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
  tabs.go/.h/.m                Tabbed container (NSTabView) with delegate callbacks
  modal.go/.h/.m               Modal dialog (NSPanel) with dismiss delegate + data binding
  video.go/.h/.m               Video player (AVPlayerView) with playback controls + onEnded
  audio.go/.h/.m               Audio player (AVPlayer) compact control bar + onEnded
  screenshot.go/.h/.m          Window capture (NSBitmapImageRep → PNG bytes)
  style.go/.h/.m               Cross-cutting visual style application (bg, color, radius, font, alignment, flexGrow)

transport/                     Message sources
  transport.go                 Transport interface (Messages, Errors, Start, Stop, SendAction)
  file.go                      FileTransport (reads JSONL from file with include support, SendAction is no-op)
  dir.go                       DirTransport (reads all *.jsonl in a directory, sorted)
  llm.go                       LLMTransport (bidirectional LLM conversation loop)
  llm_tools.go                 11 A2UI tool definitions + toolCallToMessage + inspectLibrary + system prompt
  cache.go                     Prompt caching (SHA256 hash, CachePaths, CacheValid, WriteHashFile)
  anthropic.go                 Anthropic provider with prompt caching (cache_control headers)
  file_test.go                 8 tests: channel lifecycle + include + circular detection + depth limit
  llm_test.go                  Mock provider tests: tool call parsing, transport lifecycle, action turns
  contract_test.go             RunTransportContractTests (reusable suite, includes SendActionDoesNotPanic)
  testhelper_test.go           goroutineLeakCheck, drain helpers

mcp/                           Embedded MCP server (JSON-RPC 2.0 on stdin/stdout)
  protocol.go                  MCP types (Request, Response, Tool, etc.)
  transport.go                 Stdio transport (line-delimited JSON-RPC)
  server.go                    Server routing + tool registration
  tools.go                     14 tool handlers (query, interact, data, transport, capture)
  dispatch.go                  dispatchSync generic helper for main-thread queries
  server_test.go               MCP server tests

testdata/                      JSONL fixtures (29 top-level + subdirectories)
  hello.jsonl                  Card with heading + body text
  contact_form.jsonl           Form with data binding, preview card, checkbox, submit
  contact_form_test.jsonl      Native e2e test: contact form with test cases
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
  calculator.jsonl             Full calculator with styled buttons and expression evaluator
  calculator_test.jsonl        Native e2e test: calculator with assertions
  ffi_runtime_test.jsonl       FFI: runtime loadLibrary + functionCall (typed signatures)
  ffi_test.jsonl               FFI: static --ffi-config with typed function declarations
  assets.jsonl                 Asset loading demo
  custom_functions.jsonl       defineFunction: digit buttons using appendDigit user function
  component_defs.jsonl         defineComponent: DigitButton + OpButton templates
  scoped_components.jsonl      State scoping: two Counter instances with isolated state
  tabs.jsonl                   Tabs with data-bound tab selection
  tabs_test.jsonl              Native e2e test: tabs with assertions
  flexgrow_test.jsonl          FlexGrow: Text and Column children expanding in Row
  flexlist_test.jsonl          FlexGrow in forEach List template
  modal.jsonl                  Modal dialog with data-bound visibility and dismiss
  modal_test.jsonl             Native e2e test: modal open/close via data binding
  video.jsonl                  Video player with controls and caption
  video_test.jsonl             Native e2e test: video props and children
  video_player_app.jsonl       Video Player sample app (all Video features: autoplay, mute, switch, onEnded)
  audio.jsonl                  Audio player demo with compact controls
  audio_test.jsonl             Native e2e test: audio player props and children
  audio_player_app.jsonl       Audio Player sample app (track switching, loop toggle, onEnded)
  reminders.jsonl              Full Reminders app (Tabs, List, CheckBox, Button actions)
  includes/                    Include feature: main.jsonl includes defs.jsonl
  calculator_v2/               All DX features combined: include + defineFunction + defineComponent

sample_apps/                   LLM-generated sample applications (9 apps)
  */prompt.txt                 Natural language app description (sent to LLM)
  */prompt.jsonl               Cached JSONL output (auto-generated, .gitignored except sysinfo)
  sysinfo/                     FFI demo: loads libcurl, libsqlite3, libz and displays versions
  calculator/                  Calculator app matching macOS Calculator.app style
  todo/                        Todo list app
  dashboard/                   Dashboard layout demo
  gallery/                     Image gallery
  registration/                Registration form
  settings/                    Settings panel
  theme_switcher/              Theme switching demo (light/dark via setTheme functionCall)
  scrollable_feed/             Scrollable feed demo (List with scroll view)
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
- `build/jview test <file.jsonl>` — native e2e tests with real AppKit rendering
- MockRenderer + MockDispatcher enable full engine testing without AppKit
- Native test runner uses real DarwinRenderer + synchronous MockDispatcher (avoids dispatch_async deadlock)
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
13. **Slider callback float conversion** — slider eventData arrives as a string but `surface.go` converts it to float64 via `fmt.Sscanf` before storing in DataModel. Test expectations must use numbers, not strings.
14. **Test style vs layout fields** — TestStep has separate `Layout` and `Style` fields (each `map[string]interface{}`). The `assertStyle` function reads from `step.Style`, not `step.Layout`. These must not be conflated.
15. **assertChildren/assertCount on nonexistent components** — returns a "not found" error, not a silent 0 or empty list. Tests that check count=0 must use an existing component.
16. **FFI string returns are library-owned** — `returnType: "string"` copies via `C.GoString()` but does NOT free the native pointer. The library is assumed to own the memory (static buffers, etc.). If the native code malloc'd the string, call a registered free function explicitly.
17. **FFI pointer handle table** — pointer returns become integer handle IDs. Pass handle IDs (not raw pointers) back to functions expecting `pointer` args. Invalid handle IDs produce a clear error, not a crash.
18. **libffi required** — the FFI subsystem links against `-lffi`. On macOS, libffi is in the SDK (`/Library/Developer/CommandLineTools/SDKs/MacOSX*.sdk/usr/include/ffi`). Also available via Homebrew.
19. **FFI test dylib** — `ffi_runtime_test.jsonl` and `ffi_test.jsonl` depend on `/tmp/jview_test_ffi_lib.dylib` which is only built by `go test ./engine/`. Run `make test` before `make verify` (or use `make check` which does both) for full FFI fixture rendering.
20. **ChildList dual format** — LLMs generate `"children":{"static":["a","b"]}` (object with "static" key). Hand-written JSONL uses `"children":["a","b"]` (bare array). The parser handles both.
21. **defineFunction body deep copy** — the function body is deep-copied before param substitution on every call. Without this, substitution mutates the shared definition and subsequent calls break.
22. **Component expansion order** — `expandComponentInstances` runs before `expandTemplates`. Component instances are expanded first (useComponent → inline components), then forEach templates expand. Both operate on the same component list.
23. **Component ID rewriting** — `_root` becomes the instance ID, `_X` becomes `instanceId__X`. Non-underscore IDs are left as-is. The instance's parent, style, and children (if any on the instance) override the template root's.
24. **State scoping $ prefix** — `$` in paths is replaced with the scope value. Default scope is `"/instanceId"`. The `$` replacement is recursive through all JSON values including nested functionCall args and data model ops.
25. **Include circular detection** — uses absolute path tracking. The include stack is a `map[string]bool` passed through recursive calls. Max depth is 10 to prevent accidental infinite recursion.
26. **Directory mode vs include** — directory mode reads all `*.jsonl` sorted; include reads specific files. They can be combined but files will be processed twice (with "redefining" warnings for duplicate definitions). This is harmless.
27. **NSTabView content layout** — NSTabView manages `item.view` frame via frame-based layout. The container wrapping tab content must keep `translatesAutoresizingMaskIntoConstraints = YES` (default). Setting it to NO gives zero-size content.
28. **flexGrow bypasses NSStackView distribution** — `NSStackViewDistributionFill` doesn't expand NSStackView children (intrinsicContentSize returns {-1,-1}). When any child has flexGrow, stackview.m adds children as regular subviews with manual constraint chains instead of using `addArrangedSubview`.
29. **forEach clone ID namespacing** — Template clone IDs are prefixed by the parent List's component ID (e.g. `myList_row_0` instead of `row_0`). This prevents ID collisions when multiple forEach lists share the same template.
30. **MCP server on pipe** — `jview mcp` uses real AppKit (not headless). Layout queries return real NSView frames. The file transport (if provided) loads UI before MCP client connects. Surfaces may not be available immediately after `send_message` — use `wait_for` to poll.
31. **MCP interaction thread safety** — `InvokeCallback` from MCP goroutine must be wrapped in `dispatchSync` to run on the main thread. After the callback, a second `dispatchSync` no-op flushes renders queued via `dispatch_async` (GCD serial queue is FIFO). Without this, tool returns before renders complete and queries see stale state.
32. **MCP OnAction handler** — `sess.OnAction` must be set to a no-op in MCP mode. Without it, buttons with event actions (serverAction) panic on nil function call. MCP has no transport to forward actions to.

## What To Work On Next

See `plan.md` for the full roadmap. Phase 3 is complete — all components, transport, testing, and styling are done. The next phase is Production Hardening:

1. **CGo memory cleanup** — audit and fix any leaks in the ObjC bridge
2. **Error recovery / graceful degradation** — handle malformed messages, missing components
3. **Multi-surface window management** — multiple windows, surface lifecycle
4. **Incremental tree diff** — only re-render actually changed components
5. **CLI flags + stdin transport** — pipe JSONL from stdin
6. **macOS .app bundle packaging** — distributable application

## Commands

```bash
make build                           # Build binary to build/jview
make test                            # Headless tests with -race (300+ tests)
make verify                          # Screenshot verification (31 fixtures)
make check                           # Full gate (test + verify)
build/jview test testdata/contact_form_test.jsonl  # Native e2e test
build/jview testdata/hello.jsonl     # File mode (static fixture)
build/jview testdata/calculator_v2/  # Directory mode (reads all *.jsonl sorted)
build/jview --ffi-config libs.json testdata/app.jsonl  # With FFI config
build/jview --prompt "Build a todo app"  # LLM mode (default: anthropic/haiku)
build/jview --prompt-file prompt.txt    # LLM mode with prompt from file
build/jview --llm openai --model gpt-4o --prompt "Build a calculator"
build/jview mcp testdata/hello.jsonl    # MCP server with pre-loaded UI
build/jview mcp                          # MCP server (empty, create UI via send_message)
make verify-fixture F=testdata/hello.jsonl  # Single fixture screenshot
make run-app A=sysinfo               # Run a sample app
make generate-apps                   # Generate all sample apps (headless)
make regen-app A=calculator          # Force-regenerate a sample app
```

## Environment

- **Go 1.25.0** required (for any-llm-go dependency). Install: `go install golang.org/dl/go1.25.0@latest && go1.25.0 download`
- **ANTHROPIC_API_KEY** — set for default LLM mode. Other providers use their standard env vars (OPENAI_API_KEY, etc.)
- Use `~/go/bin/go1.25.0` if system Go is older than 1.25

# jview — Native macOS A2UI Renderer

## What This Is
Go + CGo + Objective-C app that renders A2UI JSONL protocol as native Cocoa/AppKit widgets. No webview, no Electron.

## Testing & Verification

### The Gate: `make check`
```
make check
```
This is the single command that validates everything. Run it before any commit. It runs two stages in order:

1. **`make test`** — headless unit + integration tests (no display needed)
2. **`make verify`** — build binary, launch every fixture, capture screenshots

If stage 1 fails, stage 2 doesn't run. Both must pass.

### Four Layers of Testing

#### Layer 1: Unit Tests (headless, fast, CI-safe)
```
make test
```
Pure Go tests against protocol/, engine/ with mock renderer. No CGo, no AppKit, no display.

**What they cover:**
- Protocol parsing: every message type, dynamic values, child lists, edge cases
- DataModel: JSON Pointer get/set/delete, nested paths, missing keys
- BindingTracker: registration, unregistration, path overlap detection
- Tree: update, root detection, child ordering, change tracking
- Render ordering: topological sort guarantees children created before parents
- Data binding: TextField → DataModel → bound Text propagation
- CheckBox binding: toggle → DataModel → bound components
- Button callbacks: onClick fires ActionHandler with correct action name
- DataModel updates: updateDataModel message triggers re-render of bound components

**How they work:**
- `renderer/mock.go` provides `MockRenderer` (records all ops) and `MockDispatcher` (runs synchronously)
- Engine tests feed JSONL strings to Session and assert on MockRenderer's recorded state
- E2E tests read actual `testdata/*.jsonl` files through FileTransport → Session → MockRenderer

**Where to add tests:**
- New protocol feature → `protocol/parse_test.go`
- New engine logic → `engine/*_test.go` for the unit, `engine/integration_test.go` for the flow
- New component → add a case to an integration test that creates the component and checks resolved props
- New data binding pattern → `engine/integration_test.go` with InvokeCallback + assert Updated

#### Layer 2: Screenshot Verification (requires macOS display)
```
make verify
```
Builds the real binary, launches each `testdata/*.jsonl` fixture, waits 2 seconds, captures `build/screenshots/<name>.png`, kills the process.

**What it catches:** CGo bridge failures, ObjC crashes, layout bugs, visual regressions — things unit tests can't see.

**What to check in screenshots:**
- **hello.png**: Card titled "Welcome" with h1 "Hello, jview!" and body text
- **contact_form.png**: Column with h2 heading, Name/Email labels+fields, Preview card, checkbox, blue Submit button
- **layout.png**: h1 heading, horizontal Row with two Cards side-by-side

**Single fixture:**
```
make verify-fixture F=testdata/hello.jsonl
```

#### Layer 3: Native E2E Tests (real AppKit, automated)
```
build/jview test testdata/contact_form_test.jsonl
```
Runs with real `darwin.Renderer` on the main thread using synchronous `MockDispatcher` (avoids dispatch_async deadlock). Test messages (`"type":"test"`) are interleaved in the same JSONL files. Queries real NSView frames, fonts, colors.

**What they cover:**
- Component prop assertions (subset matching on resolved props)
- Data model value assertions at JSON Pointer paths
- Child relationship assertions (ordered child IDs, count)
- Action assertions (action fired with correct name/data)
- Layout assertions (real NSView frame: x, y, width, height)
- Style assertions (real NSView font, text color, background, opacity)
- Event simulation (change, click, toggle, slide, select, datechange)

**Assertion types:** `component`, `dataModel`, `children`, `notExists`, `count`, `action`, `layout`, `style`

**When to do this:** after adding components, changing layout, or modifying the rendering pipeline.

**Adding a new e2e test:**
1. Create `testdata/<name>_test.jsonl` with app setup + `"type":"test"` messages
2. Add a Go test in `engine/testrunner_test.go` that calls `RunTestFile` with the fixture
3. Run `build/jview test testdata/<name>_test.jsonl` to verify with real AppKit

#### Layer 4: Interactive Testing (manual, for binding/callback work)
```
build/jview testdata/contact_form.jsonl
```
Window stays open. Type in fields, click buttons, toggle checkboxes. Quit with Cmd+Q.

**When to do this:** after changing callback flow, data binding, or adding interactive components.

### Fixture Discipline

Every component and every feature gets a fixture in `testdata/`. Fixtures are:
- The **test data** for headless E2E tests (engine/e2e_test.go reads them)
- The **input** for screenshot verification
- The **demo** for interactive testing

**When adding a new component:**
1. Create `testdata/<component>.jsonl` with the component in a realistic layout
2. Add an E2E test in `engine/e2e_test.go` that reads the fixture and asserts on MockRenderer
3. Run `make check` — headless tests pass, screenshot captured
4. Read the screenshot — visual output is correct

**When adding a new engine feature (e.g., function calls):**
1. Create `testdata/<feature>.jsonl` exercising the feature
2. Add integration test in `engine/integration_test.go` with inline JSONL
3. Add E2E test reading the fixture
4. `make check`

### Test File Map

| File | What it tests | Layer |
|------|--------------|-------|
| `protocol/parse_test.go` | JSONL parsing, message types, dynamic values | Unit |
| `engine/datamodel_test.go` | JSON Pointer operations | Unit |
| `engine/binding_test.go` | Path → component tracking, overlap | Unit |
| `engine/tree_test.go` | Component hierarchy, roots, children | Unit |
| `engine/integration_test.go` | Session + Surface with mock renderer | Integration |
| `engine/e2e_test.go` | Full pipeline: file → transport → engine → mock | E2E |
| `engine/testrunner.go` | Native e2e test runner (real AppKit assertions) | Test infra |
| `engine/testrunner_test.go` | Test runner unit tests (16 tests) | Unit |
| `renderer/mock.go` | MockRenderer + MockDispatcher | Test infra |
| `platform/darwin/viewquery.go/.h/.m` | ObjC view frame/style queries | Test infra |
| `testdata/*.jsonl` | Fixture files used by E2E + screenshots | Data |
| `testdata/*_test.jsonl` | Native e2e test fixtures with inline assertions | E2E |
| `engine/channel_test.go` | Channel manager: create/delete, pub/sub, queue, cleanup | Unit |
| `samples/dynamic_list.jsonl` | Dynamic list with add/remove + inline tests | E2E |
| `sample_apps/*/prompt.jsonl` | Sample app cached JSONL with inline tests | E2E |

## Architecture

```
Transport (goroutine)          ← reads JSONL from file/LLM
    ↓
engine.Session (goroutine)     ← routes messages to surfaces
    ↓
engine.Surface                 ← manages tree, data model, bindings
    ↓
Dispatcher.RunOnMain()         ← batches render ops to main thread
    ↓
darwin.Renderer (main thread)  ← CGo → ObjC → NSView creation/updates
    ↓
Native Cocoa widgets           ← visible on screen
```

## Layer Rules

| Layer | May import | Must NOT import |
|-------|-----------|----------------|
| protocol/ | stdlib only | engine, renderer, platform |
| engine/ | protocol, renderer | platform |
| renderer/ | protocol | engine, platform |
| platform/darwin/ | protocol, renderer | engine |
| transport/ | protocol | engine, renderer, platform |
| mcp/ | protocol, engine, renderer | platform, transport |
| main.go | everything | — |

## CGo Conventions
- Every `.go` file with `import "C"` needs `#cgo CFLAGS: -x objective-c -fobjc-arc`
- Each component = 3 files: `widget.go` + `widget.h` + `widget.m`
- `callback.go` needs `#include <stdint.h>` for `C.uint64_t`
- `cgo.Handle` is integer — pass to `C.uintptr_t` directly, never wrap in `unsafe.Pointer`
- Use `objc_setAssociatedObject` to prevent target-action dealloc

## Rendering Rules
- Topological sort: create leaves before parents (children before containers)
- Two-pass: (1) create/update all views, (2) set children on containers
- Root view: pin top/leading/trailing tight, bottom with `constraintLessThanOrEqualToAnchor` (or `=` if root has flexGrow children, via `kJVNeedsFlexExpansionKey` flag)
- NSBox (Card): add stack to existing contentView, never replace contentView

## Callback Flow
1. Engine registers callback via `rend.RegisterCallback()` → gets CallbackID
2. CallbackID stored in `RenderNode.Callbacks` map
3. Component bridge reads from `node.Callbacks` during `CreateView`
4. ObjC target calls `GoCallbackInvoke(callbackID, data)` → globalRegistry → Go func
5. Two-way binding: callback writes to DataModel → BindingTracker finds affected components → re-render (skip self)

## Adding a New Component

1. Add type constant to `protocol/component.go`
2. Add props fields to `protocol.Props`
3. Add resolved fields to `renderer.ResolvedProps`
4. Add resolver case in `engine/resolver.go`
5. Create `platform/darwin/widget.go` + `.h` + `.m`
6. Add switch cases in `darwin.DarwinRenderer.CreateView`, `UpdateView`, `SetChildren`
7. Add callback registration in `engine/surface.go` if interactive
8. Add testdata fixture, `make verify-fixture F=testdata/new.jsonl`, read screenshot

## Roadmap

Full roadmap tracked in planning MCP (plan: `jview Roadmap`). Summary:

### Phase 1: MVP — COMPLETE
Protocol parsing, engine core, 7 component bridges (Text, Row, Column, Card, Button, TextField, CheckBox), file transport, Makefile verify pipeline.

### Phase 2: Full Interactivity + Remaining Components — COMPLETE
FunctionCall evaluator (17 built-in functions incl. array: append, removeLast, slice), validation, template expansion, 7 new components (Divider, Icon, Image, Slider, ChoicePicker, DateTimeInput, List).

### Phase 3: Media + Live Transport + Polish — COMPLETE
Live agent connectivity and remaining A2UI components.

| Task | Tag | Priority | Status |
|------|-----|----------|--------|
| LLM transport (any-llm-go) | transport | critical | **done** |
| Action response pipeline | transport | high | **done** |
| Native e2e test framework | testing | high | **done** |
| Tabs | component | high | **done** |
| Embedded MCP server | infra | high | **done** |
| Modal | component | high | **done** |
| Video (AVPlayerView) | component | medium | **done** |
| AudioPlayer | component | low | **done** |
| Theme → NSAppearance | infra | low | **done** |
| Scroll view for overflow | infra | medium | **done** |

### Phase 4: Production Hardening — COMPLETE
Reliability, process model, channels, always-on MCP.

| Task | Tag | Priority | Status |
|------|-----|----------|--------|
| CGo memory cleanup | infra | high | **done** |
| Error recovery / graceful degradation | infra | high | **done** |
| Process model (createProcess/stopProcess) | infra | high | **done** |
| Channel primitives (pub/sub, broadcast/queue) | infra | high | **done** |
| Always-on MCP server | infra | high | **done** |
| macOS .app bundle packaging | infra | low | not started |

### Notes Clone — COMPLETE
4 new native components, 3 new evaluator functions, Apple Notes sample app.

| Task | Tag | Priority | Status |
|------|-----|----------|--------|
| SplitView (NSSplitView) | component | high | **done** |
| OutlineView (NSOutlineView) | component | high | **done** |
| SearchField (NSSearchField) | component | high | **done** |
| RichTextEditor (NSTextView) | component | high | **done** |
| filter/find/getField functions | engine | high | **done** |
| Notes sample app (3-pane layout) | app | high | **done** |

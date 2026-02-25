# jview A2UI Protocol Specification

This document describes the A2UI JSONL protocol subset implemented by jview and the rendering rules applied by the engine.

## Wire Format

Messages are newline-delimited JSON (JSONL). Each line is a self-contained JSON object. Blank lines are skipped. Maximum line size: 10MB.

Every message has a `type` field and most have a `surfaceId` field:

```json
{"type":"<messageType>","surfaceId":"<id>", ...}
```

## Message Types

### createSurface

Opens a new native window.

```json  
{
  "type": "createSurface",
  "surfaceId": "main",
  "title": "My App",
  "width": 800,
  "height": 600
}
```

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| surfaceId | string | yes | | Unique identifier for this surface |
| title | string | yes | | Window title |
| width | int | no | 800 | Window width in points |
| height | int | no | 600 | Window height in points |
| backgroundColor | string | no | system | Window background color (`#RRGGBB`) |
| padding | int | no | 20 | Root view inset in points (-1 for edge-to-edge) |

Duplicate `createSurface` for the same `surfaceId` is silently ignored.

### deleteSurface

Closes and removes a surface.

```json
{
  "type": "deleteSurface",
  "surfaceId": "main"
}
```

Deleting a non-existent surface is a no-op.

### updateComponents

Sends a batch of component definitions. Components are created or replaced atomically.

```json
{
  "type": "updateComponents",
  "surfaceId": "main",
  "components": [
    {"componentId": "t1", "type": "Text", "props": {"content": "Hello"}},
    {"componentId": "col", "type": "Column", "children": ["t1"]}
  ]
}
```

| Field | Type | Description |
|-------|------|-------------|
| components | Component[] | Array of component definitions |

Components within a batch may reference each other as children. The engine topologically sorts them (leaves first) before rendering.

### updateDataModel

Applies JSON Patch-style operations to the surface's data model.

```json
{
  "type": "updateDataModel",
  "surfaceId": "main",
  "ops": [
    {"op": "add", "path": "/name", "value": "Alice"},
    {"op": "replace", "path": "/count", "value": 42},
    {"op": "remove", "path": "/temp"}
  ]
}
```

| Op | Description |
|----|-------------|
| add | Set value at path, creating intermediate objects |
| replace | Same as add (idempotent) |
| remove | Delete value at path |

Paths use JSON Pointer syntax (RFC 6901): `/foo/bar/0` addresses `root.foo.bar[0]`.

The `"-"` token in a path (e.g. `/items/-`) appends to the end of an array, following JSON Patch (RFC 6902) conventions.

Op values for `add`/`replace` support the full expression language — path references and function calls are resolved through the evaluator before being stored. This enables computed updates like `{"functionCall": {"name": "add", "args": [{"path": "/counter"}, 1]}}`.

After all ops execute, the engine finds components bound to affected paths and re-renders them.

### test

Defines an inline test case with assertions and event simulations. Test messages are interleaved with app messages in the same JSONL file. `jview <file.jsonl>` ignores test messages. `jview test <file.jsonl>` executes them against real AppKit rendering.

```json
{
  "type": "test",
  "surfaceId": "main",
  "name": "initial state",
  "steps": [
    {"assert": "component", "componentId": "heading", "props": {"content": "Welcome", "variant": "h1"}},
    {"assert": "dataModel", "path": "/name", "value": ""},
    {"assert": "children", "componentId": "root", "children": ["heading", "body"]},
    {"assert": "count", "componentId": "root", "count": 2},
    {"assert": "notExists", "componentId": "ghost"},
    {"assert": "layout", "componentId": "heading", "layout": {"width": 200}},
    {"assert": "style", "componentId": "heading", "style": {"fontSize": 24, "bold": true}}
  ]
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| surfaceId | string | yes | Surface to test against |
| name | string | yes | Human-readable test case name |
| steps | TestStep[] | yes | Sequence of assertions and simulations |

#### Assertion Types

| Assert | Fields | Description |
|--------|--------|-------------|
| `component` | componentId, props, componentType | Subset match on resolved props. Optionally check component type. |
| `dataModel` | path, value | Check data model value at JSON Pointer path |
| `children` | componentId, children | Check ordered list of child component IDs |
| `count` | componentId, count | Check number of children |
| `notExists` | componentId | Verify component does not exist |
| `action` | name, data | Check that an action was fired with matching name and data |
| `layout` | componentId, layout | Check computed layout (x, y, width, height) from real NSView frames |
| `style` | componentId, style | Check computed style (fontName, fontSize, bold, italic, textColor, bgColor, hidden, opacity) from real NSView properties |

#### Event Simulation

```json
{"simulate": "event", "componentId": "nameField", "event": "change", "eventData": "Alice"}
```

| Event | Component | Description |
|-------|-----------|-------------|
| `change` | TextField | Set text value |
| `click` | Button | Trigger onClick action |
| `toggle` | CheckBox | Toggle checked state |
| `slide` | Slider | Set slider value |
| `select` | ChoicePicker | Select option |
| `datechange` | DateTimeInput | Set date value |
| `ended` | Video, AudioPlayer | Fire onEnded callback |

#### Test Runner Behavior

- Tests execute sequentially in file order
- Side effects from simulations persist across tests (shared session state)
- Captured actions reset at the start of each test
- `jview test` uses real AppKit rendering (not mocked) with synchronous dispatch
- Exit code 0 if all pass, 1 if any fail

### loadLibrary

Loads a native dynamic library at runtime and registers its exported functions for use in component expressions via `functionCall`. Uses libffi for generic function invocation — any C function with any signature can be called directly, no wrappers needed.

```json
{
  "type": "loadLibrary",
  "path": "libcurl.dylib",
  "prefix": "curl",
  "functions": [
    {"name": "version", "symbol": "curl_version", "returnType": "string", "paramTypes": []},
    {"name": "init", "symbol": "curl_easy_init", "returnType": "pointer", "paramTypes": []},
    {"name": "perform", "symbol": "curl_easy_perform", "returnType": "int", "paramTypes": ["pointer"]},
    {"name": "cleanup", "symbol": "curl_easy_cleanup", "returnType": "void", "paramTypes": ["pointer"]}
  ]
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| path | string | yes | Path to the dynamic library (.dylib/.so/.dll). Resolved by dlopen. |
| prefix | string | yes | Namespace prefix for registered functions (e.g. `curl` → callable as `curl.version`) |
| functions | FuncDef[] | yes | Array of function declarations |

#### Function Declaration

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| name | string | yes | Function name used in expressions (prefixed: `prefix.name`) |
| symbol | string | yes | Exported C symbol name in the library |
| returnType | string | yes | C return type (see type table below) |
| paramTypes | string[] | yes | C parameter types in order (empty array for no-arg functions) |
| fixedArgs | int | no | For variadic functions: number of fixed parameters before the `...` part |

#### Supported Types

| Type | C equivalent | JSON representation |
|------|-------------|---------------------|
| `void` | void | null (return only) |
| `int` | int32_t | number |
| `uint32` | uint32_t | number |
| `int64` | int64_t | number |
| `uint64` | uint64_t | number |
| `float` | float | number |
| `double` | double | number |
| `string` | const char* | string |
| `bool` | int (0/1) | boolean |
| `pointer` | void* | number (handle ID) |

#### Pointer Handle Table

Functions returning `pointer` register the raw pointer in an internal handle table and return an integer handle ID. Pass that handle ID as an argument to functions expecting a `pointer` parameter — the engine resolves it back to the actual pointer. This allows safe opaque pointer management across function calls (e.g. `init` → handle → `perform(handle)` → `cleanup(handle)`).

#### String Return Convention

`returnType: "string"` returns a Go string copied from the native `char*`. The native memory is assumed library-owned and is NOT freed by the engine.

Multiple `loadLibrary` messages can be sent. Libraries persist for the session lifetime. The FFI registry is propagated to all existing surfaces after loading.

`loadLibrary` does not require a `surfaceId` — it operates at the session level.

### defineFunction

Registers a reusable parametric function for use in expressions. Defined functions are available in `functionCall` nodes just like built-in functions. Operates at session level (no surfaceId).

```json
{
  "type": "defineFunction",
  "name": "appendDigit",
  "params": ["current", "digit"],
  "body": {"functionCall": {"name": "concat", "args": [{"param": "current"}, {"param": "digit"}]}}
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| name | string | yes | Function name (used in `functionCall.name`) |
| params | string[] | yes | Parameter names |
| body | any | yes | Expression tree with `{"param":"name"}` placeholders |

The body is deep-copied and parameters are substituted at call time. Custom functions are checked after built-ins but before FFI. Arity is enforced — wrong arg count produces an error.

### defineComponent

Registers a reusable component template. Instances are expanded inline before rendering. Operates at session level (no surfaceId).

```json
{
  "type": "defineComponent",
  "name": "DigitButton",
  "params": ["digit"],
  "components": [
    {"componentId": "_root", "type": "Button", "props": {
      "label": {"param": "digit"},
      "onClick": {"action": {"functionCall": {"call": "updateDataModel", "args": {
        "ops": [{"op": "replace", "path": "/display", "value": {"functionCall": {"name": "appendDigit", "args": [{"path": "/display"}, {"param": "digit"}]}}}]
      }}}}
    }}
  ]
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| name | string | yes | Component name (used in `useComponent`) |
| params | string[] | yes | Parameter names |
| components | Component[] | yes | Template components. Must include `_root`. |

#### ID Rewriting Convention

- `_root` → replaced with the instance's `componentId`
- `_X` (any ID starting with `_`) → replaced with `instanceId__X`
- Non-underscore IDs are left as-is

#### Component Instance

To use a defined component, set `useComponent` instead of `type`:

```json
{"componentId": "btn7", "useComponent": "DigitButton", "args": {"digit": "7"}}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| useComponent | string | yes | Name of defined component |
| args | object | no | Arguments matching the component's params |
| scope | string | no | Data model path prefix for `$` paths (default: `/instanceId`) |

#### State Scoping

Component templates can use `$` as a path prefix placeholder. During expansion, `$` is replaced with the instance's `scope` value:

```json
// In template:
"content": {"path": "$/count"}

// Instance with scope="/c1":
"content": {"path": "/c1/count"}
```

This enables multiple instances of the same component with isolated state.

### include

Includes another JSONL file at the transport level. The included file is read and its messages are injected in-place. Operates at transport level (no surfaceId).

```json
{"type": "include", "path": "defs.jsonl"}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| path | string | yes | Relative path to JSONL file (resolved from current file's directory) |

Includes are recursive (max depth 10). Circular includes are detected by absolute path and produce an error. Include messages are consumed by the transport — the engine never sees them.

#### Directory Mode

When the argument to jview is a directory instead of a file, all `*.jsonl` files in that directory are read in sorted (lexicographic) order:

```bash
build/jview testdata/calculator_v2/
# Reads: components.jsonl, functions.jsonl, main.jsonl (sorted)
```

### setTheme

Changes the visual theme for a surface's window via NSAppearance.

```json
{
  "type": "setTheme",
  "surfaceId": "main",
  "theme": "dark"
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| surfaceId | string | yes | Surface whose window to restyle |
| theme | string | yes | `"light"`, `"dark"`, or `"system"` |

Setting appearance after views exist triggers recursive layer invalidation to flush cached rendering. Can also be invoked as a client-side `functionCall` action (see [Built-in FunctionCall Actions](#built-in-functioncall-actions)).

### createProcess

Creates a named background process with its own transport. The process routes messages through the shared session. Process status is automatically written to `/processes/{id}/status` in the data model of all surfaces.

```json
{
  "type": "createProcess",
  "processId": "ticker",
  "transport": {
    "type": "interval",
    "interval": 1000,
    "message": {"type": "updateDataModel", "surfaceId": "main", "ops": [
      {"op": "replace", "path": "/counter", "value": {"functionCall": {"name": "add", "args": [{"path": "/counter"}, 1]}}}
    ]}
  }
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| processId | string | yes | Unique identifier for this process |
| transport | ProcessTransportConfig | yes | Transport configuration (see below) |

#### Process Transport Types

| Type | Fields | Description |
|------|--------|-------------|
| `file` | `path` | Reads JSONL from a file asynchronously |
| `interval` | `interval`, `message` | Sends a fixed message on a timer (ms) |
| `llm` | `provider`, `model`, `prompt` | Starts a new LLM conversation |

Process status values: `"running"`, `"stopped"`, `"error"`. Binding to `/processes/{id}/status` enables reactive UI updates when process state changes.

Creating a process with a duplicate `processId` returns an error.

### stopProcess

Terminates a running process.

```json
{
  "type": "stopProcess",
  "processId": "ticker"
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| processId | string | yes | ID of the process to stop |

Stopping a non-existent process returns an error. The process status is set to `"stopped"` in the data model.

### sendToProcess

Routes a message to a process. The inner message is parsed and handled by the session.

```json
{
  "type": "sendToProcess",
  "processId": "agent",
  "message": {"type": "updateDataModel", "surfaceId": "main", "ops": [
    {"op": "replace", "path": "/response", "value": "Hello"}
  ]}
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| processId | string | yes | Target process ID |
| message | object | yes | A2UI JSONL message to route |

### createChannel

Registers a named channel for inter-process communication. Channels provide pub/sub messaging with broadcast or queue semantics. Published values are automatically written to `/channels/{id}/value` in the data model of all surfaces.

```json
{
  "type": "createChannel",
  "channelId": "notifications",
  "mode": "broadcast"
}
```

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| channelId | string | yes | | Unique channel identifier |
| mode | string | no | `"broadcast"` | `"broadcast"` (all subscribers) or `"queue"` (round-robin) |
| bufferSize | int | no | 0 | Reserved for future use |

Creating a channel with a duplicate `channelId` returns an error. An unknown mode logs a warning and defaults to broadcast.

Channel status is written to `/channels/{id}/status` on all surfaces (`"active"` on create, `"deleted"` on delete).

### deleteChannel

Removes a channel and all its subscriptions.

```json
{
  "type": "deleteChannel",
  "channelId": "notifications"
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| channelId | string | yes | Channel to delete |

Deleting a non-existent channel returns an error.

### subscribe

Registers interest in a channel's values. When a value is published to the channel, it is delivered to the subscriber's `targetPath` in the data model.

```json
{
  "type": "subscribe",
  "channelId": "notifications",
  "processId": "worker1",
  "targetPath": "/ui/notification"
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| channelId | string | yes | Channel to subscribe to |
| processId | string | no | Subscriber identifier (empty = session-level) |
| targetPath | string | no | Data model path to deliver values to |

Duplicate subscriptions (same processId + targetPath) are deduplicated silently. Subscribing to a non-existent channel returns an error.

### unsubscribe

Removes a subscription from a channel.

```json
{
  "type": "unsubscribe",
  "channelId": "notifications",
  "processId": "worker1",
  "targetPath": "/ui/notification"
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| channelId | string | yes | Channel to unsubscribe from |
| processId | string | no | Subscriber identifier |
| targetPath | string | no | If set, only remove this specific subscription. If empty, remove all subscriptions for the processId. |

Unsubscribing from a non-existent channel returns an error.

### publish

Sends a value to a channel. The value is written to `/channels/{id}/value` on all surfaces. Subscribers receive the value at their `targetPath`.

```json
{
  "type": "publish",
  "channelId": "notifications",
  "value": {"text": "Build complete", "status": "success"}
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| channelId | string | yes | Channel to publish to |
| value | any | yes | Value to publish (any JSON type) |

**Broadcast mode:** All subscribers receive the value at their targetPath.

**Queue mode:** One subscriber receives the value (round-robin). The next publish goes to the next subscriber.

Publishing to a non-existent channel returns an error. Publishing to a channel with no subscribers is allowed — the value is still written to `/channels/{id}/value`.

---

## Component Model

### Component Definition

```json
{
  "componentId": "unique_id",
  "type": "Text",
  "props": { ... },
  "children": ["child1", "child2"]
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| componentId | string | yes | Unique ID within the surface |
| type | string | yes* | Component type name |
| props | object | no | Component-specific properties |
| children | ChildList | no | Child component references |
| useComponent | string | no | Name of a defined component (replaces `type`) |
| args | object | no | Arguments for the component template |
| scope | string | no | Data model scope prefix for `$` paths |

*`type` is required unless `useComponent` is set.

### ChildList

Either a static array of component IDs:

```json
"children": ["a", "b", "c"]
```

Or a template for dynamic expansion:

```json
"children": {
  "forEach": "/items",
  "templateId": "item_tmpl",
  "itemVariable": "item"
}
```

### Dynamic Values

Any string, number, or boolean property can be either a literal or a data model reference.

**Literal:**
```json
"content": "Hello"
```

**Path reference (resolves from data model):**
```json
"content": {"path": "/user/name"}
```

**Function call:**
```json
"content": {"functionCall": {"name": "concat", "args": ["Hello, ", {"path": "/name"}]}}
```

When a path reference is used, the engine registers a binding so the component re-renders when that data model path changes.

---

## Component Types

### Text

Displays read-only text.

| Prop | Type | Default | Values |
|------|------|---------|--------|
| content | DynamicString | `""` | Text to display |
| variant | string | `"body"` | `h1`, `h2`, `h3`, `h4`, `h5`, `body`, `caption` |

### Row

Horizontal stack layout (NSStackView, horizontal).

| Prop | Type | Default | Description |
|------|------|---------|-------------|
| justify | string | | `start`, `center`, `end`, `spaceBetween`, `spaceAround` |
| align | string | | `start`, `center`, `end`, `stretch` |
| gap | int | 0 | Spacing between children in points |
| padding | int | 0 | Internal padding in points |

### Column

Vertical stack layout (NSStackView, vertical).

Same props as Row.

### Card

Titled container (NSBox).

| Prop | Type | Default | Description |
|------|------|---------|-------------|
| title | DynamicString | `""` | Box title |
| subtitle | DynamicString | `""` | Subtitle text |
| collapsible | DynamicBoolean | `false` | Whether card can collapse |
| collapsed | DynamicBoolean | `false` | Initial collapsed state |
| padding | int | 0 | Internal padding |

### Button

Clickable button with action (event or functionCall).

| Prop | Type | Default | Description |
|------|------|---------|-------------|
| label | DynamicString | `""` | Button text |
| style | string | `"secondary"` | `primary`, `secondary`, `destructive` |
| disabled | DynamicBoolean | `false` | Whether button is disabled |
| onClick | EventAction | | Action to fire on click |

### TextField

Text input with optional two-way data binding.

| Prop | Type | Default | Description |
|------|------|---------|-------------|
| placeholder | DynamicString | `""` | Placeholder text |
| value | DynamicString | `""` | Current value |
| inputType | string | `"shortText"` | `shortText`, `longText`, `number`, `obscured` |
| readOnly | DynamicBoolean | `false` | Whether editing is disabled |
| dataBinding | string | | JSON Pointer for two-way binding |
| onChange | EventAction | | Action to fire on change |

When `dataBinding` is set, typing in the field writes the value to the data model at that path. Any components bound to overlapping paths re-render automatically (excluding the source field).

### CheckBox

Toggle with optional two-way data binding.

| Prop | Type | Default | Description |
|------|------|---------|-------------|
| label | DynamicString | `""` | Label text |
| checked | DynamicBoolean | `false` | Current state |
| dataBinding | string | | JSON Pointer for two-way binding |
| onToggle | EventAction | | Action to fire on toggle |

### Slider

Range input with optional two-way data binding.

| Prop | Type | Default | Description |
|------|------|---------|-------------|
| min | DynamicNumber | 0 | Minimum value |
| max | DynamicNumber | 100 | Maximum value |
| step | DynamicNumber | 1 | Step increment |
| sliderValue | DynamicNumber | 0 | Current value |
| dataBinding | string | | JSON Pointer for two-way binding |

### Image

Displays an image from a URL.

| Prop | Type | Default | Description |
|------|------|---------|-------------|
| src | DynamicString | `""` | Image URL |
| alt | DynamicString | `""` | Accessibility description |
| width | int | | Fixed width in points |
| height | int | | Fixed height in points |

### Icon

Displays an SF Symbol icon.

| Prop | Type | Default | Description |
|------|------|---------|-------------|
| name | DynamicString | `""` | SF Symbol name (e.g. `star.fill`) |
| size | int | 16 | Icon size in points |

### Divider

Visual separator line. No props.

### List

Scrollable container. Same layout props as Column. Children are displayed in a scroll view.

### ChoicePicker

Dropdown or segmented selection.

| Prop | Type | Default | Description |
|------|------|---------|-------------|
| options | array | `[]` | Array of `{value, label}` objects |
| dataBinding | string | | JSON Pointer for two-way binding |
| mutuallyExclusive | DynamicBoolean | `true` | Single vs multi-select |

### DateTimeInput

Date and/or time picker with two-way data binding.

| Prop | Type | Default | Description |
|------|------|---------|-------------|
| enableDate | DynamicBoolean | `true` | Show date picker |
| enableTime | DynamicBoolean | `false` | Show time picker |
| dataBinding | string | | JSON Pointer for two-way binding |

### Tabs

Tabbed container (NSTabView). Each child is a tab panel.

| Prop | Type | Default | Description |
|------|------|---------|-------------|
| tabLabels | string[] | `[]` | Tab titles (one per child) |
| activeTab | DynamicString | `""` | Component ID of the active tab panel |
| dataBinding | string | | JSON Pointer for active tab state |

Children of a Tabs component are displayed one at a time, selected by the tab bar. The `activeTab` value is the component ID of the active child. When `dataBinding` is set, tab selection writes the selected child's component ID to the data model.

### Modal

Modal dialog overlay (NSPanel). A zero-height proxy view participates in the component tree while a floating NSPanel shows the actual content.

| Prop | Type | Default | Description |
|------|------|---------|-------------|
| title | DynamicString | `""` | Panel title |
| visible | DynamicBoolean | `false` | Show/hide the panel |
| dataBinding | string | | JSON Pointer for visible state |
| width | int | 480 | Panel width in points |
| height | int | 320 | Panel height in points |
| onDismiss | EventAction | | Action to fire when the close button is clicked |

When `dataBinding` is set, dismissing the panel writes `false` to the data model path, allowing data-driven show/hide. Children are laid out in a vertical stack inside the panel.

### Video

Video playback using AVKit's AVPlayerView.

| Prop | Type | Default | Description |
|------|------|---------|-------------|
| src | DynamicString | `""` | Video URL |
| width | int | | Fixed width in points |
| height | int | | Fixed height in points |
| autoplay | DynamicBoolean | `false` | Start playing on load |
| loop | DynamicBoolean | `false` | Loop playback when video ends |
| controls | DynamicBoolean | `true` | Show native player controls |
| muted | DynamicBoolean | `false` | Mute audio |
| onEnded | EventAction | | Action to fire when playback reaches end (non-loop mode only) |

The Video component is a leaf node (no children). URL change detection avoids reloading the same video. Autoplay only applies on initial load, not on updates. Loop mode seeks to the beginning and plays again on end. The `onEnded` callback fires only when loop is false.

### AudioPlayer

Audio playback using AVFoundation's AVPlayer with a compact control bar (play/pause button, progress scrubber, time display).

| Prop | Type | Default | Description |
|------|------|---------|-------------|
| src | DynamicString | `""` | Audio URL |
| autoplay | DynamicBoolean | `false` | Start playing on load |
| loop | DynamicBoolean | `false` | Loop playback when audio ends |
| onEnded | EventAction | | Action to fire when playback reaches end (non-loop mode only) |

The AudioPlayer is a leaf node (no children). It renders as a horizontal bar ~40pt tall that stretches to parent width. Controls are always shown (play/pause, scrubber, time label). URL change detection avoids reloading the same audio. Autoplay only applies on initial load, not on updates. The `onEnded` callback fires only when loop is false.

### SplitView

Resizable multi-pane layout (NSSplitView).

| Prop | Type | Default | Description |
|------|------|---------|-------------|
| dividerStyle | string | `"thin"` | `thin`, `thick`, `paneSplitter` |
| vertical | DynamicBoolean | `true` | Vertical dividers (side-by-side panes). `false` for horizontal dividers (stacked panes). |

Children become resizable panes. Each child is wrapped in a frame-based container so NSSplitView manages pane frames while children use Auto Layout internally. On first layout, pane widths are read from children's `style.width` constraints (if set); remaining space is distributed equally to panes without explicit width. Minimum pane width is 100px, enforced by the delegate.

### OutlineView

Hierarchical tree list (NSOutlineView) with disclosure triangles and optional SF Symbol icons. Source-list style sidebar appearance.

| Prop | Type | Default | Description |
|------|------|---------|-------------|
| outlineData | DynamicString | `""` | JSON-serialized tree array |
| labelKey | string | `"name"` | Key for display text in each node |
| childrenKey | string | `"children"` | Key for nested items in each node |
| iconKey | string | `""` | Key for SF Symbol name in each node |
| idKey | string | `"id"` | Key for unique item identifier |
| selectedId | DynamicString | `""` | Currently selected item ID (data-bound) |
| dataBinding | string | | JSON Pointer for selected ID (two-way) |
| onSelect | EventAction | | Action to fire on selection change (sends selected item ID) |

The data source parses JSON tree data. Update preserves expansion state. All items expanded by default.

### SearchField

Native search input (NSSearchField) with magnifying glass icon, cancel button, and keystroke callbacks.

| Prop | Type | Default | Description |
|------|------|---------|-------------|
| placeholder | DynamicString | `""` | Placeholder text |
| value | DynamicString | `""` | Current search string |
| dataBinding | string | | JSON Pointer for two-way binding |
| onChange | EventAction | | Action to fire on each keystroke |

The built-in cancel button clears the field and fires the callback with an empty string.

### RichTextEditor

Rich text editor (NSTextView in NSScrollView) with bidirectional markdown conversion. Stores content as markdown in the data model; renders as formatted NSAttributedString.

| Prop | Type | Default | Description |
|------|------|---------|-------------|
| content | DynamicString | `""` | Markdown content |
| editable | DynamicBoolean | `true` | Whether editing is enabled |
| dataBinding | string | | JSON Pointer for content (two-way) |
| onChange | EventAction | | Action to fire on content change (debounced 300ms) |

Supported markdown: `# Title`, `## Heading`, `### Subheading`, `**bold**`, `*italic*`, `~~strikethrough~~`, `` `monospace` ``, `- [ ] checklist`, `- [x] checked`, `- bullet`, `1. numbered`. External data model updates are skipped while the user is actively editing to prevent cursor jump.

---

## Visual Styling

Any component can have a `style` object alongside `props`:

```json
{
  "componentId": "btn1",
  "type": "Button",
  "props": {"label": "Submit"},
  "style": {
    "backgroundColor": "#007AFF",
    "textColor": "#FFFFFF",
    "cornerRadius": 8,
    "fontSize": 16,
    "fontWeight": "semibold"
  }
}
```

| Property | Type | Description |
|----------|------|-------------|
| backgroundColor | string | Background color as `#RRGGBB` |
| textColor | string | Text/tint color as `#RRGGBB` |
| cornerRadius | number | Corner radius in points |
| width | number | Fixed width in points |
| height | number | Fixed height in points |
| fontSize | number | Font size in points |
| fontWeight | string | `bold`, `semibold`, `medium`, `light` |
| textAlign | string | `left`, `center`, `right` |
| opacity | number | Opacity 0.0–1.0 |
| flexGrow | number | Expand to fill remaining space in parent Row/Column (CSS-like flex-grow) |
| fontFamily | string | Custom font family name (falls back to system font if not found) |

Surface-level styling on `createSurface`:

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| backgroundColor | string | system default | Window background color as `#RRGGBB` |
| padding | int | 20 | Root view inset in points (-1 for edge-to-edge) |

---

## Actions

An action has two mutually exclusive forms: **event** (server-bound) and **functionCall** (client-bound).

### Event Action

Fires a named event to the transport (LLM, SSE, WebSocket). The engine resolves `dataRefs` from the data model and includes the values in the payload.

```json
"onClick": {
  "action": {
    "event": {
      "name": "submitForm",
      "dataRefs": ["/name", "/email"]
    }
  }
}
```

| Field | Type | Description |
|-------|------|-------------|
| event.name | string | Event identifier |
| event.context | object | Static key-value pairs sent with the event |
| event.dataRefs | string[] | Data model paths to resolve and include |

In LLM transport mode, the event is formatted as a user message and triggers a new conversation turn, allowing the LLM to respond by updating the UI.

### FunctionCall Action

Executes a client-side function. No server round-trip.

```json
"onClick": {
  "action": {
    "functionCall": {
      "call": "updateDataModel",
      "args": {
        "ops": [
          {"op": "replace", "path": "/count", "value": {"functionCall": {"name": "add", "args": [{"path": "/count"}, 1]}}}
        ]
      }
    }
  }
}
```

| Field | Type | Description |
|-------|------|-------------|
| functionCall.call | string | Function name to execute |
| functionCall.args | object | Arguments passed to the function |

#### Built-in FunctionCall Actions

| Call | Args | Description |
|------|------|-------------|
| `updateDataModel` | `{ops: [{op, path, value}]}` | Apply JSON Patch ops to the data model. Values can be dynamic (path refs, functionCalls). |
| `setTheme` | `{theme: "light"\|"dark"\|"system"}` | Switch the surface's window theme. No server round-trip. |

Op values in `updateDataModel` are resolved through the evaluator before being applied, so they support the full expression language (path references, nested function calls).

---

## Expression Language

Dynamic values (in props, action args, or data model op values) can use path references and function calls. The evaluator resolves these recursively at render time or action execution time.

### Path Reference

```json
{"path": "/user/name"}
```

Resolves to the current value at that JSON Pointer in the data model.

### Function Call

```json
{"functionCall": {"name": "concat", "args": ["Hello, ", {"path": "/name"}]}}
```

Args are resolved recursively — they can be literals, path refs, or nested function calls.

### Native FFI Functions

Functions loaded via `loadLibrary` are available in expressions using the `prefix.name` convention:

```json
{"functionCall": {"name": "curl.version", "args": []}}
{"functionCall": {"name": "z.compressBound", "args": [10000]}}
```

FFI functions are resolved through the same evaluator as built-in functions. Arguments are converted from JSON types to the declared C types, the native function is invoked via libffi, and the result is converted back to a JSON-compatible value.

### User-Defined Functions

Functions registered via `defineFunction` are available in the same expression contexts as built-in functions. They are checked after built-ins but before FFI functions. See [defineFunction](#definefunction) for details.

### Available Built-in Functions

| Function | Args | Returns | Description |
|----------|------|---------|-------------|
| `concat` | a, b, ... | string | Concatenate all args as strings |
| `add` | a, b | number | a + b |
| `subtract` | a, b | number | a - b |
| `multiply` | a, b | number | a * b |
| `divide` | a, b | number | a / b |
| `equals` | a, b | bool | Strict equality |
| `not` | a | bool | Logical negation |
| `greaterThan` | a, b | bool | a > b |
| `lessThan` | a, b | bool | a < b |
| `format` | template, args... | string | `%s`/`%d`/`%f` substitution |
| `if` | condition, trueVal, falseVal | any | Conditional (eager evaluation) |
| `or` | a, b, ... | bool | Logical or |
| `and` | a, b, ... | bool | Logical and |
| `toNumber` | val | number | Parse string to number |
| `toString` | val | string | Convert any value to string |
| `calc` | operator, left, right | number | Evaluate `+`/`-`/`*`/`/` dynamically |
| `contains` | str, substr | bool | String contains check |
| `length` | collection_or_string | number | Array element count or string character count |
| `negate` | num | number | Multiply by -1 |
| `append` | array, element | array | Append element to array |
| `removeLast` | array | array | Remove last element from array |
| `slice` | array, start, end? | array | Extract sub-array from start to end (exclusive) |
| `filter` | array, key, value | array | Return items where `item[key] == value` |
| `find` | array, key, value | any | Return first item where `item[key] == value` (nil if not found) |
| `getField` | object, fieldName | any | Extract a field from an object (nil if missing) |

---

## Data Model

Each surface has an independent data model — a JSON document addressed by JSON Pointers.

### Operations

- **Get** `pointer` — returns value at path, or not-found
- **Set** `pointer, value` — creates intermediate objects as needed, returns changed paths
- **Delete** `pointer` — removes value, shrinks arrays properly, returns changed paths

### Binding Propagation

1. Data model changes (from `updateDataModel` or user input via `dataBinding`)
2. Engine collects all changed paths
3. `BindingTracker.Affected(changedPaths)` finds components bound to overlapping paths
4. Those components are re-resolved and re-rendered
5. For user-input bindings, the source component is excluded from re-render

Path overlap: `/a` and `/a/b` overlap (parent-child). `/a` and `/b` do not.

---

## Rendering Pipeline

1. **Parse** — JSONL line to typed `Message`
2. **Route** — `Session` routes to the correct `Surface` by `surfaceId`
3. **Tree update** — `Surface.tree.Update()` stores components, returns changed IDs
4. **Resolve** — `Resolver` evaluates dynamic values against data model, registers bindings
5. **Callback registration** — interactive components get callbacks registered (old ones unregistered first)
6. **Topological sort** — changed components sorted leaves-first
7. **Dispatch to main thread** — two-pass render:
   - Pass 1: create or update each view (leaves first ensures children exist)
   - Pass 2: set children on containers
   - Set root view (single root directly, multiple roots wrapped in Column)

---

## Embedded MCP Server

The MCP server starts automatically on stdin/stdout in all modes using JSON-RPC 2.0. When running `jview file.jsonl`, the MCP server is available alongside the normal UI. `jview mcp [file.jsonl]` is a dedicated MCP-only mode that quits on EOF.

25 tools are available:

| Category | Tools |
|----------|-------|
| Query | `list_surfaces`, `get_tree`, `get_component`, `get_data_model`, `get_layout`, `get_style` |
| Interaction | `click`, `fill`, `toggle`, `interact` |
| Data | `set_data_model`, `wait_for` |
| Transport | `send_message` (send A2UI JSONL messages to create/update surfaces) |
| Capture | `take_screenshot` (PNG, base64-encoded) |
| Logging | `get_logs` |
| Processes | `list_processes`, `create_process`, `stop_process`, `send_to_process` |
| Channels | `list_channels`, `create_channel`, `delete_channel`, `publish`, `subscribe`, `unsubscribe` |

The MCP server enables programmatic UI control, testing, and integration with external agents or tools that speak MCP.

---

## Reserved Component Types (Not Yet Implemented)

No reserved component types remain. All components through the Notes Clone phase are implemented (22 total).

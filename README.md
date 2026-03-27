# jview

Describe a UI in a text prompt. Get a native macOS app. Real AppKit buttons, text fields, split views, outline views. Not a webview. Not Electron.

<p align="center">
  <img src="docs/screenshots/notes.png" width="700" alt="Notes app built with jview" />
</p>

<p align="center">
  <img src="docs/screenshots/contact_form.png" width="250" alt="Contact form" />
  &nbsp;&nbsp;
  <img src="docs/screenshots/calculator.png" width="160" alt="Calculator" />
  &nbsp;&nbsp;
  <img src="docs/screenshots/tabs.png" width="250" alt="Tabs" />
</p>

```bash
build/jview --prompt "Build a calculator with dark theme and orange operators"
```

That one line produces a native macOS window with working buttons, display, and arithmetic. The LLM figures out the layout, wires up the data model, and jview renders it as real Cocoa widgets.

## Install

macOS 13+, Go 1.25+.

```bash
git clone https://github.com/artpar/jview.git
cd jview
make build
```

Or build a proper `.app` bundle (registers `jview://` URL scheme and `.jsonl` file association):

```bash
make app          # -> build/jview.app
```

## Try it

**From a file** (no LLM needed):

```bash
build/jview testdata/contact_form.jsonl
```

**From a prompt** (needs `ANTHROPIC_API_KEY` or another provider):

```bash
build/jview --prompt "Build a todo list with add/remove and a count of remaining items"
```

**From Claude Code** (spawns a claude subprocess that builds the UI with MCP tools):

```bash
build/jview --claude-code "Build an Apple Notes clone with three-pane layout"
```

The LLM output is cached. Second run is instant. `--regenerate` forces a fresh call.

## Features

### 23 native components

Every component maps to a real AppKit class. No HTML, no CSS.

**Layout:** Row, Column, Card, SplitView, Tabs, List, Modal

**Input:** TextField, CheckBox, Slider, ChoicePicker, DateTimeInput, SearchField, Button

**Display:** Text, Icon, Image, Divider, ProgressBar

**Rich content:** RichTextEditor, OutlineView, Video, AudioPlayer

### Reactive data binding

The data model is a JSON document. Components bind to paths in it with JSON Pointers. Type in a TextField bound to `/name`, and every Text displaying `/name` updates immediately.

```json
{"componentId":"input","type":"TextField","props":{"dataBinding":"/name","placeholder":"Your name"}},
{"componentId":"greeting","type":"Text","props":{"content":{"path":"/name"}}}
```

No state management library. The engine handles propagation.

### LLM generation with 7 providers

Point jview at any LLM and describe what you want. The LLM gets 11 A2UI tools (`createSurface`, `updateComponents`, `updateDataModel`, etc.) and builds the UI through tool calls. User interactions flow back as conversation turns.

```bash
build/jview --llm openai --model gpt-4o --prompt "Build a settings panel"
build/jview --llm ollama --model llama3 --prompt-file spec.txt --mode raw
build/jview --llm gemini --model gemini-2.0-flash --prompt "Build a dashboard"
```

Providers: Anthropic, OpenAI, Gemini, Ollama, DeepSeek, Groq, Mistral.

### Live reload

Edit a `.jsonl` file, save, and the window rebuilds. No restart.

```bash
build/jview --watch testdata/contact_form.jsonl
```

Polls every 500ms. Tears down existing surfaces and re-reads from scratch.

### Reusable components and functions

Define a component once, use it many times with different parameters. State is scoped per instance.

```json
{"type":"defineComponent","name":"DigitButton","params":["digit"],"components":[
  {"componentId":"_root","type":"Button","props":{"label":{"param":"digit"}}}
]}
```
```json
{"componentId":"btn7","useComponent":"DigitButton","args":{"digit":"7"}}
```

`defineFunction` does the same for expressions. `include` splits apps across files. See `testdata/calculator_v2/` for all three working together.

Defined components persist in `~/.jview/library/` and show up in LLM prompts automatically.

### Background processes

Spawn named goroutines with their own transports. Timers, background LLM conversations, async file loading. Status lands in the data model at `/processes/{id}/status`.

```json
{"type":"createProcess","processId":"ticker","transport":{
  "type":"interval","interval":1000,
  "message":{"type":"updateDataModel","surfaceId":"main","ops":[
    {"op":"replace","path":"/counter","value":{"functionCall":{"name":"add","args":[{"path":"/counter"},1]}}}
  ]}
}}
```

Transport types: `file`, `interval`, `llm`, `claude-code`.

### Channels

Pub/sub between processes. Broadcast (all subscribers get every message) or queue (round-robin). Values land in the data model at `/channels/{id}/value`, so existing `dataBinding` just works.

```json
{"type":"createChannel","channelId":"alerts","mode":"broadcast"}
{"type":"subscribe","channelId":"alerts","targetPath":"/ui/alert"}
{"type":"publish","channelId":"alerts","value":{"text":"Deploy finished"}}
```

### MCP server (26 tools)

Every jview instance is an MCP server on stdin/stdout. Click buttons, fill text fields, read the data model, take screenshots, send raw messages. Claude Code connects through `.mcp.json` for interactive development.

```bash
build/jview mcp testdata/hello.jsonl       # dedicated MCP mode
build/jview --mcp-http localhost:8080 ...  # also on HTTP
```

Tools include: `click`, `fill`, `toggle`, `get_tree`, `get_component`, `get_data_model`, `set_data_model`, `take_screenshot`, `get_layout`, `get_style`, `send_message`, `get_logs`, `list_surfaces`, `create_process`, `create_channel`, `publish`, `subscribe`, and more.

### Native FFI

Load any `.dylib` at runtime and call its functions from component expressions. No wrappers.

```json
{"type":"loadLibrary","path":"libcurl.dylib","prefix":"curl","functions":[
  {"name":"version","symbol":"curl_version","returnType":"string","paramTypes":[]}
]}
```

The `sysinfo` sample app loads libcurl, libsqlite3, and libz, calling version functions and `compressBound` to show computed results.

## Sample apps

All in `sample_apps/`. Each has a `prompt.txt` (the human-language spec) and `prompt.jsonl` (the cached LLM output).

| App | What it shows |
|-----|---------------|
| `calculator` | defineComponent + defineFunction, dark theme, grid layout |
| `notes_llm` | Three-pane SplitView, OutlineView, RichTextEditor, search |
| `todo` | List with add/remove, data binding, item count |
| `sysinfo` | Native FFI calling libcurl, libsqlite3, libz |
| `dashboard` | Cards, stats, nested layout |
| `settings` | ChoicePicker, Slider, CheckBox, DateTimeInput |
| `theme_switcher` | setTheme, light/dark toggle |
| `scrollable_feed` | Scrollable List with 15 Card items |
| `channel_demo` | Pub/sub channels between processes |
| `live_monitor` | Background processes with interval transport |

```bash
make run-app A=calculator     # run from cache (or generate if no cache)
make regen-app A=notes_llm    # force-regenerate from LLM
```

## App directory structure

A jview app is a directory. `jview myapp/` looks for `app.jsonl` or `main.jsonl` as the entry point, falling back to alphabetical order.

```
myapp/
  app.jsonl          # entry point
  components.jsonl   # defineComponent definitions
  functions.jsonl    # defineFunction definitions
  assets/            # images, fonts, audio
  prompt.txt         # LLM prompt for regeneration
```

`app.jsonl` uses `include` to pull in the rest:
```json
{"type":"include","path":"components.jsonl"}
{"type":"include","path":"functions.jsonl"}
{"type":"createSurface","surfaceId":"main","title":"My App","width":800,"height":600}
{"type":"updateComponents","surfaceId":"main","components":[...]}
```

## How it works

```
JSONL source  -->  Transport  -->  Engine (Go)  -->  CGo  -->  AppKit (ObjC)  -->  window
                       ^               |                                            |
                       +--- user actions (clicks, input, toggles) <-----------------+
```

The engine maintains a component tree, a JSON data model, and a binding tracker. When the data model changes (user input, LLM update, process message), the binding tracker finds affected components and re-renders them. All rendering happens on the main thread through a dispatcher.

The protocol is [A2UI](https://a2ui.org) JSONL. Each line is a JSON message: `createSurface`, `updateComponents`, `updateDataModel`, etc. Any source that produces these messages can drive the UI.

## Development

```bash
make build         # build binary to build/jview
make app           # build macOS .app bundle
make test          # headless unit + integration tests (387 tests, -race)
make verify        # screenshot every fixture (48 fixtures)
make check         # test + verify (the gate before commits)
```

### Project layout

```
protocol/          JSONL parsing and message types
engine/            session, surfaces, data model, bindings, resolver, library, cache, FFI
renderer/          Renderer interface (platform-agnostic) + mock for tests
platform/darwin/   CGo + ObjC implementation of Renderer (23 components)
transport/         file, directory, watch, LLM, Claude Code, interval transports
mcp/               MCP server (JSON-RPC 2.0, stdin/stdout + HTTP)
packaging/         .app bundle resources (Info.plist)
testdata/          48 JSONL fixtures
sample_apps/       10 LLM-generated apps
```

### Testing

Unit and integration tests run headless with a mock renderer. Screenshot verification builds the real binary and captures every fixture. Native e2e tests run with real AppKit and assert on computed frames, fonts, and colors. MCP tests drive a running instance through tool calls.

```bash
make test                                    # headless (CI-safe)
make verify                                  # screenshots (needs display)
build/jview test testdata/contact_form_test.jsonl  # native e2e
```

### Platform support

macOS only for now. The engine, protocol, and transport layers are pure Go. The rendering layer is behind a [`Renderer` interface](renderer/renderer.go). Adding Linux (GTK4) or Windows (WinUI 3) means writing one package that implements that interface.

## License

MIT

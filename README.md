# jview

Native macOS renderer for the [A2UI](https://a2ui.org) JSONL protocol. No webview, no Electron — real AppKit widgets driven by declarative JSON. Connect to any LLM and let it build native UIs in real-time.

## What It Does

jview renders A2UI JSONL as native Cocoa widgets. Messages come from static files or live from an LLM — the LLM calls tools to create windows, add components, and update data, producing a native macOS UI. User interactions (button clicks, form input) flow back as conversation turns, so the LLM can update the UI in response. Native libraries can be loaded at runtime via FFI — call any C function with any signature directly from the UI layer.

```
LLM / File  -->  Transport  -->  Engine (Go)  -->  CGo bridge  -->  AppKit (Obj-C)  -->  Native UI
                     ^               |                                                      |
                     |               +-- FFI (libffi) --> any native .dylib                 |
                     |--- user actions (button clicks, form data) <-------------------------+
```

## Quick Start

**Requirements:** macOS, Go 1.25+

```bash
# Build
make build

# LLM mode (default: anthropic / claude-haiku-4-5-20251001)
ANTHROPIC_API_KEY=... build/jview --prompt "Build a todo app"

# Prompt from file (for longer prompts)
build/jview --prompt-file prompt.txt

# With a different provider/model
build/jview --llm openai --model gpt-4o --prompt "Build a calculator"
build/jview --llm ollama --model llama3 --prompt-file app-spec.txt --mode raw

# File mode (static JSONL fixtures)
build/jview testdata/hello.jsonl

# Load native libraries via FFI config
build/jview --ffi-config libs.json testdata/app.jsonl

# Run a sample app (from cache or LLM)
make run-app A=sysinfo

# Run all tests
make test

# Run native e2e tests (real AppKit rendering)
build/jview test testdata/contact_form_test.jsonl

# Full gate (tests + screenshot verification)
make check
```

## How It Works

A2UI JSONL defines surfaces (windows), components (widgets), and a data model (reactive state). jview processes these through a layered architecture:

```
Transport (goroutine)          <- LLM tool calls or file JSONL
    |
engine.Session (goroutine)     <- routes messages to surfaces
    |
engine.Surface                 <- manages tree, data model, bindings
    |
Dispatcher.RunOnMain()         <- batches render ops to main thread
    |
darwin.Renderer (main thread)  <- CGo -> ObjC -> NSView creation/updates
    |
Native Cocoa widgets           <- visible on screen
```

### LLM Transport

In LLM mode, jview connects to any supported provider via [any-llm-go](https://github.com/mozilla-ai/any-llm-go) and gives the LLM 8 A2UI tools (`createSurface`, `updateComponents`, `updateDataModel`, `deleteSurface`, `setTheme`, `test`, `loadLibrary`, `inspectLibrary`). The LLM calls these tools to build the UI, load native libraries, and define inline tests. When the user clicks a button with `dataRefs`, the referenced data model values are resolved and sent back to the LLM as a new conversation turn.

Supported providers: Anthropic, OpenAI, Gemini, Ollama, DeepSeek, Groq, Mistral.

Two modes:
- **tools** (default) — LLM uses tool calling. Preferred for models that support it.
- **raw** — LLM outputs JSONL directly in text. Fallback for models without tool support.

### Supported Components

| Component | Description |
|-----------|-------------|
| Text | Labels and headings (h1-h5, body, caption) |
| Row | Horizontal stack layout |
| Column | Vertical stack layout |
| Card | NSBox container with title |
| Button | Clickable button with server actions |
| TextField | Text input with two-way data binding |
| CheckBox | Toggle with two-way data binding |
| Slider | Numeric range input with data binding |
| Image | Async URL image loading |
| Icon | SF Symbols (macOS 11+) |
| Divider | Visual separator |
| List | Scrollable templated list |
| ChoicePicker | Dropdown selection |
| DateTimeInput | Date/time picker |

### Native FFI

Load any native dynamic library at runtime and call its functions directly from component expressions — no C wrappers needed. Uses libffi for generic function invocation with full type support.

```json
{"type":"loadLibrary","path":"libcurl.dylib","prefix":"curl","functions":[
  {"name":"version","symbol":"curl_version","returnType":"string","paramTypes":[]}
]}
```

Then use in components: `"content": {"functionCall": {"name": "curl.version", "args": []}}`

Supported types: void, int, uint32, int64, uint64, float, double, pointer, string, bool. Pointer returns are managed via a handle table — pass handle IDs back to functions expecting pointer args.

### Data Binding

Components can bind to the data model using JSON Pointers. When a user types in a TextField bound to `/name`, any Text component displaying `{"path": "/name"}` updates automatically.

## Example

### LLM-generated UI

```bash
# Inline prompt
build/jview --prompt "Build a simple counter with increment and decrement buttons"

# Or from a file
build/jview --prompt-file my-app-spec.txt
```

The LLM creates a window, initializes the data model, and renders components — all via tool calls.

### Static fixture

`testdata/hello.jsonl`:
```json
{"type":"createSurface","surfaceId":"main","title":"Hello jview","width":600,"height":400}
{"type":"updateComponents","surfaceId":"main","components":[
  {"componentId":"card1","type":"Card","props":{"title":"Welcome"},"children":["heading","body"]},
  {"componentId":"heading","type":"Text","props":{"content":"Hello, jview!","variant":"h1"}},
  {"componentId":"body","type":"Text","props":{"content":"Native macOS rendering.","variant":"body"}}
]}
```

## Project Structure

```
protocol/          JSONL parsing, message types, dynamic values
engine/            Session, Surface, DataModel, BindingTracker, Resolver, FFI registry
renderer/          Platform-agnostic Renderer interface + mock for tests
platform/darwin/   CGo + Objective-C AppKit implementation
transport/         Message sources (file, LLM; future: SSE, WebSocket)
testdata/          JSONL fixtures for testing and demos
sample_apps/       LLM-generated sample applications (prompt.txt -> cached prompt.jsonl)
```

## Testing

Four layers:

- **Unit tests** — pure Go, no display needed: protocol parsing, data model, bindings, resolver
- **Integration tests** — engine with mock renderer: component creation, data binding, callbacks
- **Native e2e tests** — real AppKit rendering with assertions on computed layout, style, data model, and actions
- **Screenshot verification** — builds real binary, launches fixtures, captures screenshots

All tests run with `-race` detection enabled.

```bash
make test          # Headless unit + integration tests (249 tests)
make verify        # Build + screenshot capture for all fixtures (17 fixtures)
make check         # Both (the gate)

# Native e2e tests (real AppKit, no display needed)
build/jview test testdata/contact_form_test.jsonl

# Sample apps
make run-app A=sysinfo           # Run from cache or LLM
make generate-app A=calculator   # Generate without opening window
make regen-app A=todo            # Force-regenerate from LLM
```

Native e2e tests use `test` messages interleaved in JSONL files. They run with real `darwin.Renderer` and query actual NSView frames, fonts, and colors. See [spec.md](spec.md#test) for the full test message format.

## License

MIT

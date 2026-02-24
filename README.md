# jview

Native macOS renderer for the [A2UI](https://a2ui.org) JSONL protocol. No webview, no Electron — real AppKit widgets driven by declarative JSON. Connect to any LLM and let it build native UIs in real-time.

## What It Does

jview renders A2UI JSONL as native Cocoa widgets. Messages come from static files or live from an LLM — the LLM calls tools to create windows, add components, and update data, producing a native macOS UI. User interactions (button clicks, form input) flow back as conversation turns, so the LLM can update the UI in response.

```
LLM / File  -->  Transport  -->  Engine (Go)  -->  CGo bridge  -->  AppKit (Obj-C)  -->  Native UI
                     ^                                                                      |
                     |--- user actions (button clicks, form data) <-------------------------+
```

## Quick Start

**Requirements:** macOS, Go 1.25+

```bash
# Build
make build

# LLM mode (default: anthropic / claude-haiku-4-5-20251001)
ANTHROPIC_API_KEY=... build/jview --prompt "Build a todo app"

# With a different provider/model
build/jview --llm openai --model gpt-4o --prompt "Build a calculator"
build/jview --llm ollama --model llama3 --prompt "Build a form" --mode raw

# File mode (static JSONL fixtures)
build/jview testdata/hello.jsonl

# Run all tests
make test

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

In LLM mode, jview connects to any supported provider via [any-llm-go](https://github.com/mozilla-ai/any-llm-go) and gives the LLM 5 A2UI tools (`createSurface`, `updateComponents`, `updateDataModel`, `deleteSurface`, `setTheme`). The LLM calls these tools to build the UI. When the user clicks a button with `dataRefs`, the referenced data model values are resolved and sent back to the LLM as a new conversation turn.

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

### Data Binding

Components can bind to the data model using JSON Pointers. When a user types in a TextField bound to `/name`, any Text component displaying `{"path": "/name"}` updates automatically.

## Example

### LLM-generated UI

```bash
build/jview --prompt "Build a simple counter with increment and decrement buttons"
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
engine/            Session, Surface, DataModel, BindingTracker, Resolver
renderer/          Platform-agnostic Renderer interface + mock for tests
platform/darwin/   CGo + Objective-C AppKit implementation
transport/         Message sources (file, LLM; future: SSE, WebSocket)
testdata/          JSONL fixtures for testing and demos
```

## Testing

Three layers:

- **Unit tests** — pure Go, no display needed: protocol parsing, data model, bindings, resolver
- **Integration tests** — engine with mock renderer: component creation, data binding, callbacks
- **Screenshot verification** — builds real binary, launches fixtures, captures screenshots

All tests run with `-race` detection enabled.

```bash
make test          # Headless unit + integration tests
make verify        # Build + screenshot capture for all fixtures
make check         # Both (the gate)
```

## License

MIT

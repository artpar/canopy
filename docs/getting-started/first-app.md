---
layout: default
title: Your First App
parent: Getting Started
nav_order: 2
---

# Your First App

## Running a JSONL File

The simplest way to use Canopy is to run an existing JSONL file:

```bash
build/canopy testdata/contact_form.jsonl
```

![Contact Form](../screenshots/contact_form.png){: .screenshot}

The window stays open until you quit with Cmd+Q.

## The JSONL Format

A Canopy app is a sequence of JSON messages, one per line. Each message tells the engine what to create or update. Here is a minimal hello world:

```jsonl
{"type":"createSurface","surfaceId":"main","title":"Hello","width":400,"height":200}
{"type":"updateComponents","surfaceId":"main","components":[{"componentId":"greeting","type":"Text","props":{"content":"Hello, world!","variant":"h1"}}]}
```

Save that as `hello.jsonl` and run it:

```bash
build/canopy hello.jsonl
```

### What Each Line Does

**Line 1 --- `createSurface`** creates a native window:

```json
{
  "type": "createSurface",
  "surfaceId": "main",
  "title": "Hello",
  "width": 400,
  "height": 200
}
```

- `surfaceId` is the unique ID for this window
- `title`, `width`, `height` set the window chrome

**Line 2 --- `updateComponents`** populates the window with components:

```json
{
  "type": "updateComponents",
  "surfaceId": "main",
  "components": [
    {
      "componentId": "greeting",
      "type": "Text",
      "props": {
        "content": "Hello, world!",
        "variant": "h1"
      }
    }
  ]
}
```

- Each component has a `componentId`, a `type`, and `props`
- Components without a parent become root elements in the window
- Container components (Row, Column, Card) list their children in a `children` array

## Adding More Components

Here is a more complete example with a card containing a heading and body text:

```jsonl
{"type":"createSurface","surfaceId":"main","title":"Welcome","width":600,"height":400}
{"type":"updateComponents","surfaceId":"main","components":[{"componentId":"card1","type":"Card","props":{"title":"Welcome"},"children":["heading","body"]},{"componentId":"heading","type":"Text","props":{"content":"Hello, Canopy!","variant":"h1"}},{"componentId":"body","type":"Text","props":{"content":"This is a native macOS app built from a text file.","variant":"body"}}]}
```

Components reference each other by ID. The Card lists `["heading","body"]` as children, so those Text components render inside the card.

## Generating from a Prompt

Instead of writing JSONL by hand, describe what you want and let an LLM generate it:

```bash
build/canopy --prompt "Build a todo list with add and delete buttons"
```

This sends your description to the configured LLM provider (Anthropic by default), receives JSONL back, and renders the result as a native window. The generated JSONL is cached, so running the same prompt again opens instantly.

To use a different provider or model:

```bash
build/canopy --prompt "Build a calculator" --llm openai --model gpt-4o
build/canopy --prompt "Build a settings panel" --llm ollama --model llama3
```

## Using Claude Code

For more complex apps, Claude Code mode gives the LLM access to MCP tools so it can iteratively build and refine the UI:

```bash
build/canopy --claude-code "Build a notes app with a sidebar, search, and rich text editor"
```

This spawns a Claude subprocess that can inspect the running app, click buttons, fill fields, and adjust the layout until the result matches your description.

## Running Sample Apps

Canopy ships with sample apps in `sample_apps/`. Run one with:

```bash
make run-app A=calculator
make run-app A=notes
make run-app A=todo
```

These use cached JSONL when available, or generate from the prompt file on first run.

## Bundling Your App

Once your app is ready, bundle it into a standalone macOS `.app`:

```bash
canopy bundle myapp/
```

The resulting `.app` is self-contained --- double-click to launch, no CLI needed. Add `--sign` for codesigning or `--notarize` for Gatekeeper-free distribution. See the **[Bundling & Distribution guide](/canopy/guide/bundling/)** for the full workflow.

## Next Steps

- **[CLI Reference](cli-reference/)** --- All modes, flags, and make targets
- **[Components](/canopy/components/)** --- The full catalog of 25 native widgets
- **[Building Apps](/canopy/guides/building-apps/)** --- Data binding, callbacks, layouts, and validation

---
layout: default
title: Vision
nav_order: 0
---

# The Vision: From Text to Native

## The Problem

Building native applications is hard. Building native *macOS* applications is harder. You need Xcode, Interface Builder, AppKit knowledge, Swift or Objective-C, signing certificates, and weeks of iteration. The result is beautiful — real buttons, real text fields, real split views — but the cost is enormous.

The web solved distribution but introduced a different problem: every app is a browser tab wearing a costume. Electron wraps Chromium in a native shell, but you're still rendering HTML in a web engine. The buttons aren't real buttons. The text fields aren't real text fields. The app doesn't feel like it belongs on your machine, because it doesn't — it belongs on the web.

What if there was a middle path? What if you could describe what you want in plain text and get a real native app — not a webview, not a simulation, but actual Cocoa widgets rendered by AppKit?

## The Insight

Large language models can generate structured data. They can produce JSON, JSONL, XML — any format you teach them. If you give an LLM a protocol for describing user interfaces, it can generate a complete app definition: windows, components, layout, data binding, interactions, styling.

The missing piece was never the AI. It was the renderer — something that takes that structured description and turns it into real native widgets, with real two-way data binding, real keyboard shortcuts, real dark mode support.

That's Canopy.

## The Protocol: A2UI

A2UI (AI-to-UI) is a JSONL protocol. Each line is a self-contained message. Messages create windows, define components, update data, wire up interactions:

```json
{
  "type": "createSurface",
  "surfaceId": "main",
  "title": "My App"
}

{
  "type": "updateComponents",
  "surfaceId": "main",
  "components": [
    {
      "componentId": "root",
      "type": "Column",
      "children": [
        "greeting"
      ]
    },
    {
      "componentId": "greeting",
      "type": "Text",
      "props": {
        "content": "Hello, world",
        "variant": "h1"
      }
    }
  ]
}
```

Two lines. A native macOS window with a heading. No Xcode. No Swift. No build step.

The protocol is deliberately simple. A human can read it. An LLM can write it. A transport can stream it. The complexity lives in the renderer, not the protocol.

### Why JSONL?

- **Streamable.** An LLM generates tokens left-to-right. JSONL lets the renderer process each message as it arrives — the window appears and fills in progressively, not all-at-once after generation completes.
- **Composable.** Files can `include` other files. Directories of JSONL files are read in order. Components can be defined once and reused.
- **Inspectable.** Every message is valid JSON. You can `cat` a file and understand what the app does. You can edit it in any text editor.
- **Transport-agnostic.** The same JSONL works from a file, from an LLM stream, from a WebSocket, from stdin. The renderer doesn't care where the messages come from.

## The Renderer: Canopy

Canopy reads A2UI JSONL and renders it as native macOS widgets:

| A2UI Component | What You Get |
|----------------|-------------|
| `Text` | NSTextField (non-editable) with variant sizing |
| `TextField` | NSTextField with placeholder, validation, two-way binding |
| `Button` | NSButton with primary/secondary/destructive styling |
| `Column` / `Row` | NSStackView with gap, padding, alignment |
| `Card` | NSBox with title, subtitle, collapsible |
| `SplitView` | NSSplitView with resizable panes |
| `Tabs` | NSTabView with data-bound tab selection |
| `OutlineView` | NSOutlineView with disclosure triangles, icons, badges |
| `RichTextEditor` | NSTextView with markdown roundtrip |
| `Video` | AVPlayerView with controls, loop, autoplay |
| `CameraView` | AVCaptureVideoPreviewLayer with photo capture |

25 components total. Every one is a real AppKit widget. When you type in a text field, NSTextField handles the input. When you click a button, NSButton fires. When you resize a split view, NSSplitView manages the panes. The operating system does what it was designed to do.

### Reactive, Not Imperative

Canopy apps don't have event loops. They have a data model — a JSON document addressed by JSON Pointers — and bindings that connect components to paths in that model.

```json
{
  "componentId": "name",
  "type": "TextField",
  "props": {
    "placeholder": "Your name",
    "dataBinding": "/user/name"
  }
}

{
  "componentId": "greeting",
  "type": "Text",
  "props": {
    "content": {
      "functionCall": {
        "name": "concat",
        "args": [
          "Hello, ",
          {
            "path": "/user/name"
          }
        ]
      }
    }
  }
}
```

Type in the text field → the data model updates at `/user/name` → the greeting text re-renders. No event handlers, no state management, no boilerplate. The engine tracks bindings and propagates changes automatically.

This is the key design decision: **the protocol describes what, not how.** The renderer figures out the rest.

## The Modes

Canopy isn't just a file viewer. It's a platform with multiple entry points:

### From a File
```bash
canopy myapp.jsonl
```
The simplest mode. Read JSONL, render it. Edit the file, add `--watch`, see changes live.

### From a Prompt
```bash
canopy --prompt "Build a calculator with dark theme and orange operators"
```
Canopy sends the prompt to an LLM (Anthropic, OpenAI, Gemini, Ollama, or 3 others), the LLM generates A2UI JSONL, Canopy renders it. The result is cached — run the same prompt again and it loads instantly.

### From Claude Code
```bash
canopy --claude-code "Build a notes app with sidebar, search, and rich text editing"
```
Claude Code gets an MCP connection to the running app. It can generate UI, take screenshots, click buttons, inspect the data model, and iterate until the app works. The LLM doesn't just generate — it verifies.

### As an MCP Server
```bash
canopy mcp myapp.jsonl
```
51 tools available over JSON-RPC. External agents can control the app: click buttons, fill forms, read data, capture screenshots. Canopy becomes a programmable native UI layer.

### As a Menubar App
```bash
canopy
```
No arguments — Canopy lives in the menu bar. Installed apps appear in the dropdown. Click to launch. A native app launcher for AI-generated applications.

## The Ecosystem

A single JSONL file is an app. A GitHub repo with a `canopy.json` manifest is a package:

```json
{
  "name": "notes",
  "version": "1.2.0",
  "type": "app",
  "entry": "prompt.jsonl",
  "description": "Apple Notes clone with 3-pane layout"
}
```

```bash
canopy pkg install artpar/canopy-notes
```

Packages are discovered via GitHub topic search (`canopy-package`), installed from tarballs, versioned with semver tags. Apps, reusable components, themes, and FFI configs are all packages.

The vision: **an ecosystem where anyone can publish a native macOS app by pushing JSONL to GitHub.** No App Store review. No code signing. No Xcode project. Just a manifest and a JSONL file.

## The Layers

Canopy is built in layers, each independently useful:

```
┌─────────────────────────────────────┐
│  User: "Build me a notes app"       │  ← Natural language
├─────────────────────────────────────┤
│  LLM generates A2UI JSONL           │  ← AI layer
├─────────────────────────────────────┤
│  A2UI Protocol (JSONL messages)     │  ← Protocol layer
├─────────────────────────────────────┤
│  Canopy Engine (Go)                 │  ← Engine layer
│  - Parse, resolve, bind, render     │
├─────────────────────────────────────┤
│  Native macOS (Objective-C / CGo)   │  ← Platform layer
│  - AppKit, AVKit, AVFoundation      │
├─────────────────────────────────────┤
│  macOS                              │  ← Operating system
└─────────────────────────────────────┘
```

You can enter at any layer:

- **Natural language** → LLM → JSONL → Canopy → Native (the "magic" path)
- **JSONL** → Canopy → Native (hand-authored apps, maximum control)
- **MCP** → Canopy → Native (programmatic control from external agents)

The protocol is the stable contract. Everything above it can change (better LLMs, different prompting strategies). Everything below it can change (different platforms, different renderers). The protocol stays.

## Where This Goes

### Multi-Platform
A2UI is not macOS-specific. The protocol describes components, not widgets. A Windows renderer could map `SplitView` to a split container, `OutlineView` to a tree view, `RichTextEditor` to a rich edit control. The same JSONL file could render natively on macOS, Windows, and Linux — each using its platform's native toolkit.

### Live Collaboration
The protocol is streamable. Two users could share a surface, each seeing real-time updates as the data model changes. The engine already supports this architecture — surfaces are addressed by ID, data model changes propagate to all bound components.

### App Composition
Processes and channels enable multi-app architectures. A dashboard app can spawn worker processes, each running their own JSONL, communicating through channels. The menubar app is already a launcher. The next step is orchestration — apps that compose other apps.

### AI as Runtime
Today, LLMs generate static JSONL that Canopy caches and replays. Tomorrow, the LLM stays connected — the app sends events (button clicks, form submissions) to the LLM, and the LLM responds with UI updates. The app evolves during use. Every interaction is a conversation.

This is already working with the LLM transport mode. The gap is making it fast enough and reliable enough for production use.

## The Bet

The bet is simple: **native apps should be as easy to create as web pages, without sacrificing the quality that makes native apps worth using.**

Webviews made apps easy to create but hard to love. Native toolkits made apps beautiful but hard to create. Canopy collapses this by putting a protocol between human intent and native rendering, with AI bridging the gap.

Two lines of JSONL make a window. A sentence of English makes an app. A GitHub repo makes it shareable. The rest is details — and the details are what this project is for.

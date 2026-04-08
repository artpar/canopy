---
layout: default
title: Home
nav_order: 1
permalink: /
---

<div class="hero" markdown="1">

# Canopy

Describe a UI in a text prompt. Get a native macOS app.
Real AppKit buttons, text fields, split views. Not a webview.
{: .tagline}

</div>

![Notes app](screenshots/notes.png){: .screenshot}

<div class="screenshot-row" markdown="1">

![Contact form](screenshots/contact_form.png){: .screenshot-small}
![Calculator](screenshots/calculator.png){: .screenshot-small}
![Theme switcher](screenshots/theme_switcher.png){: .screenshot-small}

</div>

## Quick Start

```bash
build/canopy --prompt "Build a calculator with dark theme"
```

Or run a JSONL file directly:

```bash
build/canopy testdata/contact_form.jsonl
```

<div class="feature-grid" markdown="1">

<div class="feature-card" markdown="1">
### 25 Native Components
Layout, input, display, rich text, media — all real AppKit widgets.
</div>

<div class="feature-card" markdown="1">
### Reactive Data Binding
JSON Pointer paths, two-way binding, automatic re-rendering.
</div>

<div class="feature-card" markdown="1">
### 7 LLM Providers
Anthropic, OpenAI, Gemini, Ollama, DeepSeek, Groq, Mistral.
</div>

<div class="feature-card" markdown="1">
### 51 MCP Tools
Control your app programmatically — click, fill, screenshot, capture.
</div>

<div class="feature-card" markdown="1">
### Package Ecosystem
Install, publish, and discover packages on GitHub.
</div>

<div class="feature-card" markdown="1">
### [App Bundling](guide/bundling/)
Bundle, sign, and notarize standalone `.app` bundles for distribution.
</div>

</div>

---

## Learn More

- **[Getting Started](getting-started/)** — Install Canopy, build your first app, and learn the CLI.
- **[Components](components/)** — Browse all 25 native AppKit components.
- **[Building Apps](guide/)** — Data binding, actions, layout, and advanced patterns.
- **[Protocol Reference](protocol/)** — Every message type in the A2UI protocol.
- **[Functions](functions/)** — 50+ built-in functions for expressions.

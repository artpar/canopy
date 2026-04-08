---
layout: default
title: Installation
parent: Getting Started
nav_order: 1
---

# Installation

## Build from Source

Clone the repository and build the binary:

```bash
git clone https://github.com/artpar/canopy.git
cd canopy
make build
```

This produces `build/canopy`, the single binary that runs everything.

## App Bundle (Optional)

Bundle any Canopy app into a standalone macOS `.app`:

```bash
canopy bundle myapp/                  # -> MyApp.app
canopy bundle -o Notes.app sample_apps/notes
```

The bundled `.app` is self-contained --- double-click to launch, no CLI needed. See the [CLI Reference](cli-reference/#bundle-command) for all options.

To build the Canopy runtime itself as a `.app` with URL scheme registration (`canopy://`) and `.jsonl` file association:

```bash
make app          # -> build/Canopy.app
```

## Set Up API Keys

If you plan to generate UIs from text prompts, export at least one provider key:

```bash
# Pick one (or more)
export ANTHROPIC_API_KEY=sk-ant-...
export OPENAI_API_KEY=sk-...
export GEMINI_API_KEY=...
export GROQ_API_KEY=gsk_...
export DEEPSEEK_API_KEY=...
export MISTRAL_API_KEY=...
```

For Ollama, no key is needed --- just have Ollama running locally.

You can also place keys in a `.env` file in the project root. Canopy reads it automatically without overriding existing environment variables.

## Verify the Installation

Run one of the included test fixtures to confirm everything works:

```bash
build/canopy testdata/contact_form.jsonl
```

A native window should appear with a contact form containing name and email fields, a preview card, a checkbox, and a submit button. Quit with Cmd+Q.

{: .tip }
> If you see the window, Canopy is working. Move on to [Your First App](first-app/) to learn what that JSONL file contains and how to write your own.

---
layout: default
title: CLI Reference
parent: Getting Started
nav_order: 3
---

# CLI Reference

## Modes

Canopy runs in several modes depending on the arguments you pass:

| Mode | Command | Description |
|:-----|:--------|:------------|
| File | `canopy app.jsonl` | Run a JSONL file as a native app |
| Directory | `canopy myapp/` | Run all `.jsonl` files in a directory |
| Prompt | `canopy --prompt "..." --llm anthropic` | Generate UI from an LLM prompt |
| Claude Code | `canopy --claude-code "..."` | Generate via Claude Code with MCP tool access |
| MCP Server | `canopy mcp [file.jsonl]` | Start as an embedded MCP server on stdin/stdout |
| Tray | `canopy` (no args) | Menubar-only mode with system tray icon |
| Package | `canopy pkg <cmd>` | Package management (install, publish, search) |
| Bundle | `canopy bundle <app-path>` | Create a standalone macOS `.app` from a Canopy app |

## Flags

All flags are optional. They modify behavior across modes.

| Flag | Default | Description |
|:-----|:--------|:------------|
| `--llm` | `anthropic` | LLM provider: `anthropic`, `openai`, `gemini`, `ollama`, `deepseek`, `groq`, `mistral` |
| `--model` | `claude-opus-4-6` | Model name to use for generation |
| `--prompt` | | Text description of the UI to build |
| `--prompt-file` | | Read prompt from a file (overrides `--prompt`) |
| `--mode` | `tools` | LLM mode: `tools` (structured) or `raw` (freeform) |
| `--api-key` | | API key (overrides environment variable) |
| `--regenerate` | `false` | Force a fresh LLM call, ignoring cached JSONL |
| `--generate-only` | `false` | Generate JSONL and exit without opening a window |
| `--claude-code` | | Prompt for Claude Code subprocess |
| `--save-component` | | Save generated UI as a reusable library component |
| `--watch` | `false` | Watch JSONL files for changes and reload automatically |
| `--ffi-config` | | Path to FFI convention file (JSON) for native function calls |
| `--mcp-http` | | Also listen for MCP over HTTP (e.g. `localhost:8080`) |

## Package Commands

Manage reusable components shared through GitHub:

| Command | Description |
|:--------|:------------|
| `canopy pkg login` | Authenticate with GitHub |
| `canopy pkg search <query>` | Search for packages |
| `canopy pkg info <owner/repo>` | Show package details |
| `canopy pkg install <owner/repo> [@version]` | Install a package |
| `canopy pkg uninstall <owner/name>` | Uninstall a package |
| `canopy pkg update [<owner/name>]` | Update one or all packages |
| `canopy pkg list` | List installed packages |
| `canopy pkg publish [path] [--repo=owner/repo]` | Publish a package to GitHub |

## Bundle Command

Create standalone macOS `.app` bundles from any Canopy app. The bundled app is self-contained --- double-click to launch, no CLI needed. See the **[Bundling & Distribution guide](/canopy/guide/bundling/)** for signing workflows, notarization setup, and entitlements details.

```bash
canopy bundle <app-path> [flags]
```

The `<app-path>` can be a directory containing JSONL files, or `owner/repo` to bundle an installed package.

**Flags:**

| Flag | Description |
|:-----|:------------|
| `--output`, `-o` | Output path for the `.app` bundle (default: `./<AppName>.app`) |
| `--name` | Override the app name (default: from `canopy.json` or directory name) |
| `--icon` | Path to an `.icns` file for the app icon |
| `--bundle-id` | Override the macOS bundle identifier (default: `com.canopy.app.<name>`) |
| `--sign` | Codesign with hardened runtime and entitlements |
| `--identity` | Signing identity (default: auto-detect `Developer ID Application` from keychain) |
| `--notarize` | Submit to Apple notarization and staple the ticket (implies `--sign`) |
| `--apple-id` | Apple ID email for notarization |
| `--team-id` | Team ID for notarization |
| `--password` | App-specific password for notarization |
| `--keychain-profile` | Stored keychain profile for notarization (alternative to apple-id/team-id/password) |

**Examples:**

```bash
# Bundle a local app directory
canopy bundle myapp/

# Bundle with a custom output path
canopy bundle -o ~/Desktop/Notes.app sample_apps/notes

# Bundle an installed package
canopy bundle artpar/calculator

# Ad-hoc sign (no Apple Developer account needed, works locally)
canopy bundle --sign --identity "-" myapp/

# Sign with Developer ID (requires Apple Developer account)
canopy bundle --sign myapp/

# Sign and notarize for distribution (no Gatekeeper warnings)
canopy bundle --sign --notarize --keychain-profile myprofile myapp/

# Notarize with explicit credentials
canopy bundle --sign --notarize --apple-id you@example.com --team-id ABC123 --password @keychain:AC_PASSWORD myapp/
```

Notarization credentials can also be set via environment variables: `CANOPY_APPLE_ID`, `CANOPY_TEAM_ID`, `CANOPY_APP_PASSWORD`, or `CANOPY_KEYCHAIN_PROFILE`.

The bundled binary auto-detects that it is inside a `.app` and launches as a normal dock application. Metadata (name, version, icon, bundle ID) is read from `canopy.json` if present, with flag overrides taking precedence.

## Make Targets

Common targets for building and running Canopy:

| Target | Description |
|:-------|:------------|
| `make build` | Build the `build/canopy` binary |
| `make app` | Build the `Canopy.app` bundle (includes URL scheme and file association) |
| `make run-app A=calculator` | Run a sample app by name |
| `make generate-app A=calculator` | Generate a sample app's JSONL without opening a window |
| `make regen-app A=calculator` | Force-regenerate a sample app from its prompt |
| `make generate-apps` | Generate all sample apps headlessly |
| `make clean-apps` | Remove all cached sample app JSONL files |

## Environment Variables

API keys can be set as environment variables or in a `.env` file in the project root:

| Variable | Provider |
|:---------|:---------|
| `ANTHROPIC_API_KEY` | Anthropic (Claude) |
| `OPENAI_API_KEY` | OpenAI (GPT) |
| `GEMINI_API_KEY` | Google (Gemini) |
| `GROQ_API_KEY` | Groq |
| `DEEPSEEK_API_KEY` | DeepSeek |
| `MISTRAL_API_KEY` | Mistral |

Ollama requires no API key --- it connects to `localhost:11434` by default.

## Examples

```bash
# Run a JSONL file
build/canopy testdata/contact_form.jsonl

# Run with file watching (auto-reload on save)
build/canopy --watch testdata/contact_form.jsonl

# Generate from a prompt
build/canopy --prompt "Build a todo list" --llm anthropic

# Generate and save without opening a window
build/canopy --prompt "Build a dashboard" --generate-only

# Use a different model
build/canopy --prompt "Build a chat app" --llm openai --model gpt-4o

# Use Ollama locally
build/canopy --prompt "Build a timer" --llm ollama --model llama3

# Generate with Claude Code (iterative, uses MCP tools)
build/canopy --claude-code "Build a notes app with sidebar and rich text"

# Start as MCP server
build/canopy mcp

# Start as MCP server with an initial JSONL file
build/canopy mcp testdata/contact_form.jsonl

# Start MCP server with HTTP endpoint
build/canopy --mcp-http localhost:8080 mcp

# Package management
canopy pkg search "todo"
canopy pkg install artpar/canopy-components
canopy pkg list

# Bundle an app into a standalone .app
canopy bundle myapp/
canopy bundle -o ~/Desktop/Notes.app sample_apps/notes
```

---
layout: default
title: Package Manifest
parent: Packages
nav_order: 1
---

# Package Manifest

Every Canopy package has a `canopy.json` file at the root of the repository. This file describes the package and tells Canopy how to install and run it.

## Example

```json
{
  "name": "notes",
  "version": "1.2.0",
  "type": "app",
  "description": "Apple Notes clone with 3-pane layout",
  "author": "artpar",
  "license": "MIT",
  "icon": "note.text",
  "entry": "prompt.jsonl",
  "keywords": [
    "notes",
    "productivity",
    "rich-text"
  ],
  "dependencies": {
    "sidebar-tree": ">=1.0.0"
  }
}
```

## Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | string | yes | Package name. Must be unique within the owner's namespace. |
| `version` | string | yes | Semantic version (e.g., `1.2.0`). |
| `type` | string | yes | One of: `app`, `component`, `theme`, `ffi-config`. |
| `description` | string | no | Short description of what the package does. |
| `author` | string | no | Author name or GitHub username. |
| `license` | string | no | License identifier (e.g., `MIT`, `Apache-2.0`). |
| `icon` | string | no | SF Symbol name for the app icon in the menubar (e.g., `note.text`, `calculator`). |
| `entry` | string | yes | Path to the main file, relative to the repo root. For apps, this is typically a `.jsonl` file. |
| `prompt` | string | no | Path to a `prompt.txt` file for LLM-driven apps. |
| `keywords` | array | no | Tags for search discovery. |
| `dependencies` | object | no | Map of package name to version constraint (e.g., `">=1.0.0"`, `"^2.0.0"`, `"1.3.0"`). |

## Package Types in Detail

### app

A complete application with its own window(s). The `entry` file is loaded as the main JSONL source.

Install location: `~/.canopy/apps/{owner}/{name}/`

### component

A reusable UI component defined with `defineComponent`. Can be imported by other apps.

Install location: `~/.canopy/library/{name}.jsonl`

### theme

A visual theme that sets colors, fonts, and spacing. Applied via the `theme` protocol message.

Install location: `~/.canopy/themes/{name}.jsonl`

### ffi-config

A JSON configuration file that describes a native library's functions for use with `loadLibrary` and `callFunction`.

Install location: `~/.canopy/ffi/{name}.json`

## Version Constraints

Dependencies use semver range syntax:

| Constraint | Meaning |
|-----------|---------|
| `1.2.0` | Exactly version 1.2.0 |
| `>=1.0.0` | Version 1.0.0 or higher |
| `^1.2.0` | Compatible with 1.2.0 (>=1.2.0, <2.0.0) |
| `~1.2.0` | Approximately 1.2.0 (>=1.2.0, <1.3.0) |

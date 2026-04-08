---
layout: default
title: Logging & Actions
parent: MCP Tools
nav_order: 11
---

# Logging & Actions

These tools help you debug your app by querying logs, polling for pending user actions, and sending AppKit selectors through the responder chain.

---

## get_logs

Query Canopy's internal log ring buffer. Useful for debugging rendering issues, callback failures, or protocol errors.

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `level` | string | no | Minimum log level: `"debug"`, `"info"`, `"warn"`, `"error"` |
| `component` | string | no | Filter by component name (e.g., `"renderer"`, `"engine"`, `"transport"`) |
| `pattern` | string | no | Regex pattern to match against log messages |
| `limit` | number | no | Maximum number of log entries to return |

**Example -- get recent errors:**
```
mcp__canopy__get_logs(level: "error", limit: 10)
```

**Example -- search for a specific issue:**
```
mcp__canopy__get_logs(pattern: "callback.*nil", component: "renderer")
```

**Example -- all debug logs from the engine:**
```
mcp__canopy__get_logs(level: "debug", component: "engine", limit: 50)
```

---

## get_pending_actions

Poll for queued user actions that are waiting for a response. When a component triggers an action (e.g., a button click with `event.name`), the action is queued until an agent or process responds.

**Parameters:** None

**Returns:** Array of pending action objects.

**Example:**
```
mcp__canopy__get_pending_actions()
```

```json
[
  {
    "actionId": "act_abc123",
    "name": "submitForm",
    "data": {
      "name": "Alice",
      "email": "alice@example.com"
    }
  }
]
```

Use `send_to_process` or `send_message` to respond to pending actions.

---

## perform_action

Send an AppKit selector through the macOS responder chain. This triggers standard macOS menu actions and system behaviors.

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `selector` | string | yes | Objective-C selector name (e.g., `"copy:"`, `"paste:"`, `"selectAll:"`) |

**Example -- trigger copy:**
```
mcp__canopy__perform_action(selector: "copy:")
```

**Example -- trigger undo:**
```
mcp__canopy__perform_action(selector: "undo:")
```

**Common selectors:**

| Selector | Action |
|----------|--------|
| `copy:` | Copy selection to clipboard |
| `paste:` | Paste from clipboard |
| `cut:` | Cut selection |
| `selectAll:` | Select all |
| `undo:` | Undo last action |
| `redo:` | Redo last undone action |
| `toggleFullScreen:` | Toggle full screen mode |
| `performMiniaturize:` | Minimize window |
| `performZoom:` | Zoom window |

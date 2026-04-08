---
layout: default
title: include
parent: Protocol Reference
nav_order: 9
---

# include

Includes another JSONL file, inlining its messages at the current position in the stream.

## Example

```json
{
  "type": "include",
  "path": "components/sidebar.jsonl"
}
```

## Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `type` | string | yes | `"include"` |
| `path` | string | yes | Path to the JSONL file to include |

## Behavior

- The file is read and its messages are processed in order, as if they appeared inline.
- Paths are resolved relative to the including file's directory.
- Includes can be nested (an included file can include other files).
- Maximum nesting depth is **10** to prevent infinite recursion.
- If the file does not exist, an error is logged and processing continues.

## Use Cases

- Split a large app into modular files (one per screen or feature).
- Share component definitions across multiple apps.
- Load a standard set of `defineComponent` and `defineFunction` messages.

## Example: Modular App

**main.jsonl:**
```json
{
  "type": "include",
  "path": "lib/components.jsonl"
}

{
  "type": "include",
  "path": "lib/functions.jsonl"
}

{
  "type": "createSurface",
  "surfaceId": "main",
  "title": "My App",
  "width": 800,
  "height": 600
}

{
  "type": "include",
  "path": "screens/home.jsonl"
}
```

**lib/components.jsonl:**
```json
{"type":"defineComponent","name":"NavButton","params":["label","target"],"components":[...]}
```

**screens/home.jsonl:**
```json
{"type":"updateComponents","surfaceId":"main","components":[...]}
```

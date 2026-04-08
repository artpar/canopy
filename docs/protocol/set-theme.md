---
layout: default
title: setTheme
parent: Protocol Reference
nav_order: 5
---

# setTheme

Changes the visual theme for a surface's window.

## Example

```json
{
  "type": "setTheme",
  "surfaceId": "main",
  "theme": "dark"
}
```

## Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `type` | string | yes | `"setTheme"` |
| `surfaceId` | string | yes | Target surface ID |
| `theme` | string | yes | `"light"`, `"dark"`, or `"system"` |

## Behavior

- Sets the NSAppearance for the window.
- `"system"` follows the macOS system appearance setting.
- Takes effect immediately on the window and all its components.
- Can also be triggered as a [functionCall action](../guide/actions#settheme).

## Related

- [createSurface](create-surface) -- set initial theme with the `theme` field
- [Theming guide](../guide/theming) -- theme toggle examples

---
layout: default
title: updateWindow
parent: Protocol Reference
nav_order: 20
---

# updateWindow

Modifies properties of an existing window after creation.

## Example

```json
{
  "type": "updateWindow",
  "surfaceId": "main",
  "title": "Updated Title",
  "minWidth": 400,
  "minHeight": 300
}
```

## Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `type` | string | yes | `"updateWindow"` |
| `surfaceId` | string | yes | Target surface ID |
| `title` | string | no | New window title |
| `minWidth` | int | no | Minimum window width in points |
| `minHeight` | int | no | Minimum window height in points |

## Behavior

- Only the specified fields are updated; others remain unchanged.
- `minWidth` and `minHeight` set the minimum size the user can resize the window to.
- If the surface does not exist, the message is ignored.

## Related

- [createSurface](create-surface) -- set initial window properties

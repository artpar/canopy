---
layout: default
title: deleteSurface
parent: Protocol Reference
nav_order: 2
---

# deleteSurface

Closes a window and cleans up all its components, bindings, and callbacks.

## Example

```json
{
  "type": "deleteSurface",
  "surfaceId": "main"
}
```

## Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `type` | string | yes | `"deleteSurface"` |
| `surfaceId` | string | yes | ID of the surface to close |

## Behavior

- Removes all components from the surface's tree.
- Unregisters all callbacks and data bindings.
- Destroys the native NSWindow.
- If the surface does not exist, the message is ignored.

## Related

- [createSurface](create-surface) -- open a window

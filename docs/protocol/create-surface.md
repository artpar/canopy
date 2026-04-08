---
layout: default
title: createSurface
parent: Protocol Reference
nav_order: 1
---

# createSurface

Opens a new native window (NSWindow).

## Example

```json
{
  "type": "createSurface",
  "surfaceId": "main",
  "title": "My App",
  "width": 800,
  "height": 600,
  "backgroundColor": "#FFFFFF",
  "padding": 20
}
```

## Fields

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `type` | string | yes | -- | `"createSurface"` |
| `surfaceId` | string | yes | -- | Unique identifier for the surface |
| `title` | string | no | `""` | Window title |
| `width` | int | no | system default | Window width in points |
| `height` | int | no | system default | Window height in points |
| `backgroundColor` | string | no | system default | Window background color (hex `#RRGGBB`) |
| `padding` | int | no | `0` | Internal padding around root content in points |
| `theme` | string | no | `"system"` | Initial theme: `"light"`, `"dark"`, or `"system"` |

## Behavior

- Creates a new NSWindow with the specified dimensions and title.
- The surface is immediately ready to receive `updateComponents` and `updateDataModel` messages.
- If a surface with the same `surfaceId` already exists, the message is ignored.
- The window is centered on screen by default.
- `backgroundColor` sets the window's content view background.
- `padding` adds spacing between the window edge and the root component.

## Related

- [deleteSurface](delete-surface) -- close the window
- [updateWindow](update-window) -- modify window properties after creation
- [setTheme](set-theme) -- change theme after creation

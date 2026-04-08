---
layout: default
title: updateToolbar
parent: Protocol Reference
nav_order: 19
---

# updateToolbar

Defines a toolbar for a surface's window.

## Example

```json
{
  "type": "updateToolbar",
  "surfaceId": "main",
  "items": [
    {
      "id": "add",
      "icon": "plus",
      "label": "Add",
      "action": {
        "event": {
          "name": "addItem"
        }
      },
      "bordered": true
    },
    {
      "separator": true
    },
    {
      "flexible": true
    },
    {
      "id": "search",
      "searchField": true,
      "dataBinding": "/searchQuery"
    }
  ]
}
```

## Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `type` | string | yes | `"updateToolbar"` |
| `surfaceId` | string | yes | Target surface ID |
| `items` | array | yes | Array of toolbar item specs |

### Toolbar Item

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `id` | string | no | Unique item identifier |
| `icon` | string | no | SF Symbol name |
| `label` | string | no | Tooltip or text label |
| `standardAction` | string | no | AppKit selector |
| `action` | object | no | Custom event action |
| `separator` | bool | no | Thin divider |
| `flexible` | bool | no | Flexible space (pushes items apart) |
| `searchField` | bool | no | Renders as NSSearchToolbarItem |
| `dataBinding` | string | no | JSON Pointer for search field binding |
| `enabled` | dynamic bool | no | Interactive state (default true) |
| `selected` | dynamic bool | no | Toggle/highlight state |
| `bordered` | bool | no | Rounded button appearance (macOS 11+) |

## Behavior

- Replaces the window's entire toolbar.
- Search fields support two-way data binding via `dataBinding`.
- `flexible` space items push surrounding items apart.
- `bordered` items render with a rounded button appearance.
- Toolbar items with `enabled` set to false appear grayed out.
- `selected` provides a visual toggle state.

## Related

- [updateMenu](update-menu) -- define menu bar items

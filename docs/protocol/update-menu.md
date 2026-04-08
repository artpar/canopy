---
layout: default
title: updateMenu
parent: Protocol Reference
nav_order: 18
---

# updateMenu

Defines the menu bar for a surface's window.

## Example

```json
{
  "type": "updateMenu",
  "surfaceId": "main",
  "items": [
    {
      "id": "file",
      "label": "File",
      "children": [
        {
          "id": "new",
          "label": "New",
          "keyEquivalent": "n",
          "action": {
            "event": {
              "name": "newFile"
            }
          }
        },
        {
          "id": "open",
          "label": "Open...",
          "keyEquivalent": "o",
          "action": {
            "event": {
              "name": "openFile"
            }
          }
        },
        {
          "separator": true
        },
        {
          "id": "save",
          "label": "Save",
          "keyEquivalent": "s",
          "action": {
            "event": {
              "name": "saveFile"
            }
          }
        }
      ]
    },
    {
      "id": "edit",
      "label": "Edit",
      "children": [
        {
          "id": "undo",
          "label": "Undo",
          "keyEquivalent": "z",
          "standardAction": "undo:"
        },
        {
          "id": "redo",
          "label": "Redo",
          "keyEquivalent": "z",
          "keyModifiers": "shift",
          "standardAction": "redo:"
        },
        {
          "separator": true
        },
        {
          "id": "cut",
          "label": "Cut",
          "keyEquivalent": "x",
          "standardAction": "cut:"
        },
        {
          "id": "copy",
          "label": "Copy",
          "keyEquivalent": "c",
          "standardAction": "copy:"
        },
        {
          "id": "paste",
          "label": "Paste",
          "keyEquivalent": "v",
          "standardAction": "paste:"
        }
      ]
    }
  ]
}
```

## Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `type` | string | yes | `"updateMenu"` |
| `surfaceId` | string | yes | Target surface ID |
| `items` | array | yes | Array of MenuItem objects |

### MenuItem

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `id` | string | no | Unique menu item identifier |
| `label` | string | no | Display text |
| `keyEquivalent` | string | no | Keyboard shortcut key (Cmd is always included) |
| `keyModifiers` | string | no | Additional modifiers: `"option"`, `"shift"`, `"option+shift"` |
| `separator` | bool | no | If true, renders as a separator line |
| `standardAction` | string | no | AppKit selector (e.g., `"copy:"`, `"paste:"`, `"undo:"`) |
| `action` | object | no | Custom event action |
| `children` | array | no | Submenu items |
| `icon` | string | no | SF Symbol name |
| `disabled` | dynamic bool | no | Gray out when true |

## Behavior

- Replaces the window's entire menu bar.
- Standard actions route through the AppKit responder chain.
- Custom actions fire events to the server.
- Keyboard shortcuts always include Cmd; use `keyModifiers` for additional modifiers.

## Related

- [updateToolbar](update-toolbar) -- define toolbar items

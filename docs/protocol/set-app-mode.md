---
layout: default
title: setAppMode
parent: Protocol Reference
nav_order: 21
---

# setAppMode

Switches the application's activation policy between normal, menubar, and accessory modes.

## Example

```json
{
  "type": "setAppMode",
  "mode": "menubar",
  "icon": "bolt.fill",
  "title": "My App"
}
```

With menu items:

```json
{
  "type": "setAppMode",
  "mode": "menubar",
  "icon": "bolt.fill",
  "menuItems": [
    {
      "id": "show",
      "label": "Show Window"
    },
    {
      "separator": true
    },
    {
      "id": "quit",
      "label": "Quit",
      "keyEquivalent": "q"
    }
  ]
}
```

## Fields

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `type` | string | yes | -- | `"setAppMode"` |
| `mode` | string | yes | -- | `"normal"`, `"menubar"`, or `"accessory"` |
| `icon` | string | no | -- | SF Symbol name for the status bar icon (menubar mode) |
| `title` | string | no | -- | Text for the status bar item (menubar mode) |
| `menuItems` | array | no | -- | Dropdown items for the status bar menu (menubar mode) |

## Modes

| Mode | Dock Icon | Menu Bar Item | Behavior |
|------|-----------|---------------|----------|
| `normal` | Yes | No | Standard macOS app |
| `menubar` | No | Yes | Status bar item, click toggles window |
| `accessory` | No | No | Background only |

## Behavior

- In `menubar` mode, the app stays alive when all windows are closed.
- Clicking the status bar icon toggles the window's visibility.
- `icon` accepts any SF Symbol name (e.g., `"bolt.fill"`, `"timer"`, `"gear"`).
- `menuItems` follows the same MenuItem structure as [updateMenu](update-menu).
- You can switch modes at any time.

## Related

- [App Modes guide](../guide/app-modes) -- examples and patterns
- [updateMenu](update-menu) -- MenuItem structure reference

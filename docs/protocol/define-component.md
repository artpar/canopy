---
layout: default
title: defineComponent
parent: Protocol Reference
nav_order: 8
---

# defineComponent

Registers a reusable component template that can be instantiated with `useComponent`.

## Example

```json
{
  "type": "defineComponent",
  "name": "IconButton",
  "params": [
    "icon",
    "label",
    "action"
  ],
  "components": [
    {
      "componentId": "_root",
      "type": "Row",
      "props": {
        "gap": 8,
        "align": "center"
      },
      "children": [
        "_icon",
        "_text"
      ]
    },
    {
      "componentId": "_icon",
      "type": "Icon",
      "props": {
        "name": {
          "param": "icon"
        }
      }
    },
    {
      "componentId": "_text",
      "type": "Text",
      "props": {
        "content": {
          "param": "label"
        }
      }
    }
  ]
}
```

## Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `type` | string | yes | `"defineComponent"` |
| `name` | string | yes | Template name (used in `useComponent`) |
| `params` | array | yes | Parameter names |
| `components` | array | yes | Component tree with `{"param": "name"}` placeholders |

## Behavior

- The template is registered globally and available to all surfaces.
- **`_root` is required** -- it becomes the top-level view of each instance.
- Other component IDs starting with `_` are rewritten to `{instanceId}__{suffix}` to avoid collisions.
- `{"param": "name"}` placeholders in props are replaced with argument values at instantiation time.
- Parameters prefixed with `$` create scoped data model paths.

## Instantiating

In an `updateComponents` message:

```json
{
  "componentId": "saveBtn",
  "useComponent": "IconButton",
  "args": {
    "icon": "square.and.arrow.down",
    "label": "Save"
  }
}
```

The `_root` component becomes `saveBtn`, `_icon` becomes `saveBtn__icon`, and `_text` becomes `saveBtn__text`.

## Related

- [Reusable Components guide](../guide/reusable-components) -- full examples with state scoping
- [defineFunction](define-function) -- reusable functions

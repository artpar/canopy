---
layout: default
title: updateComponents
parent: Protocol Reference
nav_order: 3
---

# updateComponents

Creates or updates components within a surface. This is the primary message for building your UI.

## Example

```json
{
  "type": "updateComponents",
  "surfaceId": "main",
  "components": [
    {
      "componentId": "root",
      "type": "Column",
      "props": {
        "gap": 16
      },
      "children": [
        "title",
        "btn"
      ]
    },
    {
      "componentId": "title",
      "type": "Text",
      "props": {
        "content": "Hello",
        "variant": "h1"
      }
    },
    {
      "componentId": "btn",
      "type": "Button",
      "props": {
        "label": "Click Me",
        "style": "primary"
      }
    }
  ]
}
```

## Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `type` | string | yes | `"updateComponents"` |
| `surfaceId` | string | yes | Target surface ID |
| `components` | array | yes | Array of component definitions |

### Component Definition

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `componentId` | string | yes | Unique ID within the surface |
| `type` | string | yes* | Component type (e.g., `"Text"`, `"Button"`, `"Column"`) |
| `props` | object | no | Component-specific properties |
| `style` | object | no | Visual style overrides |
| `children` | array | no | Ordered list of child component IDs |
| `parentId` | string | no | Alternative to `children` -- specifies parent |
| `useComponent` | string | no* | Name of a defined component template to instantiate |
| `args` | object | no | Arguments for `useComponent` |

*Either `type` or `useComponent` is required.

## Behavior

- Components are **buffered** across consecutive `updateComponents` messages and rendered as a single batch when a different message type arrives (or the stream ends).
- **Topological sort**: components are created leaf-first, ensuring children exist before their parents reference them.
- **Two-pass rendering**: (1) create/update all views, (2) set children on containers.
- If a component with the same `componentId` already exists, it is **updated** rather than recreated.
- Components no longer referenced by any parent are **pruned** (removed from the tree and cleaned up).
- Component instances (via `useComponent`) are expanded before rendering.

## Children

You can specify parent-child relationships in two ways:

**Children array** (preferred):
```json
{
  "componentId": "row",
  "type": "Row",
  "children": [
    "a",
    "b",
    "c"
  ]
}
```

**parentId**:
```json
{
  "componentId": "a",
  "type": "Text",
  "parentId": "row",
  "props": {
    "content": "A"
  }
}
```

## Related

- [defineComponent](define-component) -- create reusable templates
- [updateDataModel](update-data-model) -- initialize data before components

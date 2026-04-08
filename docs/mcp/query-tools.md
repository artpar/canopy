---
layout: default
title: Inspecting Your App
parent: MCP Tools
nav_order: 1
---

# Inspecting Your App

These tools let you read the current state of your Canopy application -- what windows are open, what components exist, their properties, layout frames, and visual styles.

---

## list_surfaces

List all open windows (surfaces) in the application.

**Parameters:** None

**Returns:** Array of surface objects with their IDs and metadata.

**Example:**
```
mcp__canopy__list_surfaces()
```

```json
[
  {
    "id": "main",
    "title": "My App",
    "width": 800,
    "height": 600
  }
]
```

---

## get_tree

Get the full component tree of a window. Returns every component with its type, props, and children.

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `surface_id` | string | yes | The window to inspect |

**Returns:** Nested tree of all components.

**Example:**
```
mcp__canopy__get_tree(surface_id: "main")
```

```json
{
  "id": "root",
  "type": "Column",
  "children": [
    {
      "id": "title",
      "type": "Text",
      "props": {
        "content": "Hello"
      }
    },
    {
      "id": "btn",
      "type": "Button",
      "props": {
        "label": "Click me"
      }
    }
  ]
}
```

---

## get_component

Get the properties of a single component.

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `surface_id` | string | yes | The window containing the component |
| `component_id` | string | yes | The component to inspect |

**Returns:** Component object with resolved props.

**Example:**
```
mcp__canopy__get_component(surface_id: "main", component_id: "email-field")
```

```json
{
  "id": "email-field",
  "type": "TextField",
  "props": {
    "label": "Email",
    "value": "user@example.com",
    "placeholder": "Enter email"
  }
}
```

---

## get_data_model

Read a value from the surface's data model at a JSON Pointer path.

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `surface_id` | string | yes | The window to query |
| `path` | string | yes | JSON Pointer path (e.g., `/form/name`, `/items/0/title`) |

**Returns:** The value at that path (string, number, object, or array).

**Example:**
```
mcp__canopy__get_data_model(surface_id: "main", path: "/form/name")
```

```json
"Alice"
```

---

## get_layout

Get the native NSView frame of a component -- its position and size in screen coordinates.

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `surface_id` | string | yes | The window containing the component |
| `component_id` | string | yes | The component to measure |

**Returns:** Frame object with x, y, width, and height.

**Example:**
```
mcp__canopy__get_layout(surface_id: "main", component_id: "sidebar")
```

```json
{
  "x": 0,
  "y": 0,
  "width": 250,
  "height": 600
}
```

---

## get_style

Get the visual style of a component -- font, text color, background color, and opacity.

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `surface_id` | string | yes | The window containing the component |
| `component_id` | string | yes | The component to inspect |

**Returns:** Style object with font, colors, and opacity.

**Example:**
```
mcp__canopy__get_style(surface_id: "main", component_id: "heading")
```

```json
{
  "font": {
    "name": "System Bold",
    "size": 24
  },
  "textColor": "#000000",
  "backgroundColor": "#FFFFFF",
  "opacity": 1.0
}
```

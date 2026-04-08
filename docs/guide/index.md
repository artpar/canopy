---
layout: default
title: Building Apps
nav_order: 3
has_children: true
---

# Building Apps

Canopy apps are JSONL files -- one JSON message per line. Each message creates a window, defines components, updates data, or wires up interactions. You send messages; Canopy renders native macOS widgets.

A minimal app needs two messages:

```json
{
  "type": "createSurface",
  "surfaceId": "main",
  "title": "My App",
  "width": 400,
  "height": 300
}

{
  "type": "updateComponents",
  "surfaceId": "main",
  "components": [
    {
      "componentId": "hello",
      "type": "Text",
      "props": {
        "content": "Hello, world!",
        "variant": "h1"
      }
    }
  ]
}
```

The first line opens a window. The second line puts a heading in it.

From here, the core concepts build on each other:

1. **[Data Binding](data-binding)** -- Connect components to a shared data model so changes propagate automatically.
2. **[Expressions](expressions)** -- Compute values dynamically using path references and function calls.
3. **[Actions](actions)** -- Respond to user input with events (server-bound) or function calls (client-bound).
4. **[Styling](styling)** -- Control colors, fonts, sizes, and layout with the `style` object.
5. **[Reusable Components](reusable-components)** -- Define component templates and instantiate them with parameters.
6. **[Reusable Functions](reusable-functions)** -- Define parametric functions for repeated logic.
7. **[Processes](processes)** -- Spawn background workers with their own transports.
8. **[Channels](channels)** -- Publish/subscribe communication between processes.
9. **[Theming](theming)** -- Switch between light, dark, and system themes.
10. **[App Modes](app-modes)** -- Run as a normal app, menubar tray, or background process.
11. **[Validation](validation)** -- Validate form fields with built-in rules.
12. **[Drag and Drop](drag-and-drop)** -- Accept file and text drops on any component.
13. **[Bundling & Distribution](bundling)** -- Create standalone `.app` bundles with codesigning and notarization.

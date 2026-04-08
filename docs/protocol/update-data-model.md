---
layout: default
title: updateDataModel
parent: Protocol Reference
nav_order: 4
---

# updateDataModel

Applies operations to a surface's data model. The data model is a JSON document that components can read from and bind to.

## Example

```json
{
  "type": "updateDataModel",
  "surfaceId": "main",
  "ops": [
    {
      "op": "add",
      "path": "/user",
      "value": {
        "name": "Alice",
        "email": "alice@example.com"
      }
    },
    {
      "op": "replace",
      "path": "/count",
      "value": 5
    },
    {
      "op": "remove",
      "path": "/temp"
    }
  ]
}
```

## Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `type` | string | yes | `"updateDataModel"` |
| `surfaceId` | string | yes | Target surface ID |
| `ops` | array | yes | Array of operations |

### Operation

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `op` | string | yes | `"add"`, `"replace"`, or `"remove"` |
| `path` | string | yes | JSON Pointer to the target location |
| `value` | any | no | Value to set (not used for `remove`) |

## Operations

### add

Sets a value at the path, creating intermediate objects as needed:

```json
{
  "op": "add",
  "path": "/user/name",
  "value": "Alice"
}
```

Use `/-` to append to an array:

```json
{
  "op": "add",
  "path": "/items/-",
  "value": {
    "title": "New item"
  }
}
```

### replace

Updates an existing value:

```json
{
  "op": "replace",
  "path": "/count",
  "value": 10
}
```

### remove

Deletes a value:

```json
{
  "op": "remove",
  "path": "/temp"
}
```

## Expression Values

Values can be expressions (path references or function calls) that are resolved before applying:

```json
{
  "op": "replace",
  "path": "/total",
  "value": {
    "functionCall": {
      "name": "add",
      "args": [
        {
          "path": "/subtotal"
        },
        {
          "path": "/tax"
        }
      ]
    }
  }
}
```

## Behavior

- Operations are applied in order.
- After all operations complete, the binding tracker finds components that reference changed paths and triggers re-rendering.
- If the surface does not exist, the message is ignored.
- Sending `updateDataModel` before `updateComponents` is the standard way to initialize state.

## Related

- [Data Binding guide](../guide/data-binding) -- how components connect to the data model
- [updateComponents](update-components) -- create components that read from the data model

---
layout: default
title: Objects
parent: Functions
nav_order: 5
---

# Object Functions

Functions for reading and modifying JSON objects.

## Reference

| Function | Args | Returns | Description |
|----------|------|---------|-------------|
| `getField` | `object, fieldName` | any | Extract a field value from an object |
| `setField` | `object, key, value` | object | Return object with field set to value |
| `updateItem` | `array, idKey, idValue, field, value` | array | Update a field on the matching item in an array |

## Examples

### getField

Read a field from an object:

```json
{
  "functionCall": {
    "name": "getField",
    "args": [
      {
        "functionCall": {
          "name": "find",
          "args": [
            {
              "path": "/users"
            },
            "id",
            {
              "path": "/selectedId"
            }
          ]
        }
      },
      "name"
    ]
  }
}
```

This finds a user by ID, then extracts their name.

### setField

Return a new object with a field set:

```json
{
  "functionCall": {
    "name": "setField",
    "args": [
      {
        "path": "/user"
      },
      "lastLogin",
      {
        "functionCall": {
          "name": "now",
          "args": []
        }
      }
    ]
  }
}
```

Returns a copy of the user object with `lastLogin` set to the current timestamp. The original object is not modified.

### updateItem

Update a specific item in an array by matching on an ID field:

```json
{
  "functionCall": {
    "name": "updateItem",
    "args": [
      {
        "path": "/todos"
      },
      "id",
      {
        "path": "/selectedId"
      },
      "done",
      true
    ]
  }
}
```

This finds the item in `/todos` where `id` matches `/selectedId` and sets its `done` field to `true`. Returns the updated array.

Common pattern -- toggle a todo item:

```json
{
  "op": "replace",
  "path": "/todos",
  "value": {
    "functionCall": {
      "name": "updateItem",
      "args": [
        {
          "path": "/todos"
        },
        "id",
        "todo-1",
        "done",
        {
          "functionCall": {
            "name": "not",
            "args": [
              {
                "functionCall": {
                  "name": "getField",
                  "args": [
                    {
                      "functionCall": {
                        "name": "find",
                        "args": [
                          {
                            "path": "/todos"
                          },
                          "id",
                          "todo-1"
                        ]
                      }
                    },
                    "done"
                  ]
                }
              }
            ]
          }
        }
      ]
    }
  }
}
```

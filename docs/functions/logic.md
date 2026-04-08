---
layout: default
title: Logic
parent: Functions
nav_order: 3
---

# Logic Functions

Functions for boolean logic and comparisons.

## Reference

| Function | Args | Returns | Description |
|----------|------|---------|-------------|
| `equals` | `a, b` | bool | True if a equals b (string equality) |
| `not` | `val` | bool | Boolean negation |
| `greaterThan` | `a, b` | bool | True if a > b (numeric) |
| `lessThan` | `a, b` | bool | True if a < b (numeric) |
| `if` | `condition, trueVal, falseVal` | any | Conditional (lazy) |
| `or` | `a, b, ...` | any | Short-circuit OR (lazy) |
| `and` | `a, b, ...` | any | Short-circuit AND (lazy) |

## Lazy Evaluation

`if`, `or`, and `and` use **lazy evaluation** -- they only evaluate the arguments they need. This means:

- `if(condition, a, b)` only evaluates `a` or `b`, not both
- `or(a, b, c)` stops at the first truthy value
- `and(a, b, c)` stops at the first falsy value

This is important when one branch might reference a path that doesn't exist yet.

## Examples

### if

Choose between two values:

```json
{
  "functionCall": {
    "name": "if",
    "args": [
      {
        "functionCall": {
          "name": "greaterThan",
          "args": [
            {
              "path": "/count"
            },
            0
          ]
        }
      },
      {
        "functionCall": {
          "name": "concat",
          "args": [
            {
              "path": "/count"
            },
            " items"
          ]
        }
      },
      "No items"
    ]
  }
}
```

### equals

Check string equality:

```json
{
  "functionCall": {
    "name": "equals",
    "args": [
      {
        "path": "/status"
      },
      "active"
    ]
  }
}
```

### Conditional Styling

Use logic functions to set dynamic styles:

```json
{
  "style": {
    "backgroundColor": {
      "functionCall": {
        "name": "if",
        "args": [
          {
            "functionCall": {
              "name": "equals",
              "args": [
                {
                  "path": "/selected"
                },
                "true"
              ]
            }
          },
          "#007AFF",
          "#FFFFFF"
        ]
      }
    }
  }
}
```

### Combining Conditions

```json
{
  "functionCall": {
    "name": "and",
    "args": [
      {
        "functionCall": {
          "name": "greaterThan",
          "args": [
            {
              "path": "/age"
            },
            18
          ]
        }
      },
      {
        "functionCall": {
          "name": "not",
          "args": [
            {
              "functionCall": {
                "name": "equals",
                "args": [
                  {
                    "path": "/banned"
                  },
                  true
                ]
              }
            }
          ]
        }
      }
    ]
  }
}
```

### not

Invert a boolean:

```json
{
  "functionCall": {
    "name": "not",
    "args": [
      {
        "path": "/isHidden"
      }
    ]
  }
}
```

### or

Return the first truthy value:

```json
{
  "functionCall": {
    "name": "or",
    "args": [
      {
        "path": "/nickname"
      },
      {
        "path": "/name"
      },
      "Anonymous"
    ]
  }
}
```

If `/nickname` is empty, falls back to `/name`, then to "Anonymous".

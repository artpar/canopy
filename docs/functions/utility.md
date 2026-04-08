---
layout: default
title: Utility
parent: Functions
nav_order: 6
---

# Utility Functions

General-purpose functions for formatting, conversion, and common operations.

## Reference

| Function | Args | Returns | Description |
|----------|------|---------|-------------|
| `toString` | `val` | string | Convert any value to its string representation |
| `format` | `template, arg0, arg1, ...` | string | Replace `{0}`, `{1}`, etc. in template |
| `length` | `s` | number | String length (also works on arrays) |
| `contains` | `s, sub` | bool | True if s contains sub |
| `now` | -- | string | Current ISO 8601 timestamp |
| `uuid` | -- | string | Generate a UUID v4 string |
| `formatDateRelative` | `isoDate` | string | Format ISO date as relative string |

## Examples

### toString

Convert a number or boolean to string:

```json
{
  "functionCall": {
    "name": "toString",
    "args": [
      {
        "path": "/count"
      }
    ]
  }
}
```

### format

Template with positional arguments:

```json
{
  "functionCall": {
    "name": "format",
    "args": [
      "Order #{0}: {1} items, ${2}",
      {
        "path": "/orderId"
      },
      {
        "path": "/itemCount"
      },
      {
        "path": "/total"
      }
    ]
  }
}
```

Produces something like `"Order #42: 3 items, $59.99"`.

### now

Get the current timestamp:

```json
{
  "functionCall": {
    "name": "now",
    "args": []
  }
}
```

Returns an ISO 8601 string like `"2025-01-15T14:30:00Z"`.

### uuid

Generate a unique identifier:

```json
{
  "functionCall": {
    "name": "uuid",
    "args": []
  }
}
```

Returns a string like `"a1b2c3d4-e5f6-7890-abcd-ef1234567890"`.

Useful when creating new items that need unique IDs:

```json
{
  "op": "replace",
  "path": "/items",
  "value": {
    "functionCall": {
      "name": "append",
      "args": [
        {
          "path": "/items"
        },
        {
          "id": {
            "functionCall": {
              "name": "uuid",
              "args": []
            }
          },
          "title": "New Item"
        }
      ]
    }
  }
}
```

### formatDateRelative

Format a date relative to now:

```json
{
  "functionCall": {
    "name": "formatDateRelative",
    "args": [
      {
        "path": "/item/createdAt"
      }
    ]
  }
}
```

Returns human-readable strings like:
- "Today at 2:30 PM"
- "Yesterday"
- "Feb 24"
- "Jan 15, 2024"

### length

Get the length of a string or array:

```json
{
  "functionCall": {
    "name": "length",
    "args": [
      {
        "path": "/items"
      }
    ]
  }
}
```

### contains

Check for a substring:

```json
{
  "functionCall": {
    "name": "contains",
    "args": [
      {
        "path": "/text"
      },
      "error"
    ]
  }
}
```

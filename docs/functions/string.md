---
layout: default
title: String
parent: Functions
nav_order: 1
---

# String Functions

Functions for manipulating text values.

## Reference

| Function | Args | Returns | Description |
|----------|------|---------|-------------|
| `concat` | `a, b, ...` | string | Concatenate all arguments as strings |
| `toUpperCase` | `s` | string | Convert to uppercase |
| `toLowerCase` | `s` | string | Convert to lowercase |
| `trim` | `s` | string | Strip leading and trailing whitespace |
| `substring` | `s, start, end?` | string | Extract substring from start index to end (exclusive) |
| `substringAfter` | `s, delimiter` | string | Return the part after the first occurrence of delimiter |
| `replace` | `s, old, new` | string | Replace all occurrences of old with new |
| `format` | `template, arg0, arg1, ...` | string | Replace `{0}`, `{1}`, etc. in template |
| `contains` | `s, sub` | bool | True if s contains sub |

## Examples

### concat

Join values into a single string:

```json
{
  "functionCall": {
    "name": "concat",
    "args": [
      "Hello, ",
      {
        "path": "/name"
      },
      "!"
    ]
  }
}
```

Accepts any number of arguments. Non-string values are converted automatically.

### substring

Extract part of a string by index:

```json
{
  "functionCall": {
    "name": "substring",
    "args": [
      {
        "path": "/text"
      },
      0,
      5
    ]
  }
}
```

The `end` argument is optional. If omitted, extracts to the end of the string.

### substringAfter

Get the part after a delimiter:

```json
{
  "functionCall": {
    "name": "substringAfter",
    "args": [
      "user@example.com",
      "@"
    ]
  }
}
```

Returns `"example.com"`.

### replace

Replace all occurrences:

```json
{
  "functionCall": {
    "name": "replace",
    "args": [
      {
        "path": "/text"
      },
      "-",
      " "
    ]
  }
}
```

### format

Template string with positional placeholders:

```json
{
  "functionCall": {
    "name": "format",
    "args": [
      "{0} has {1} items",
      {
        "path": "/name"
      },
      {
        "path": "/count"
      }
    ]
  }
}
```

### contains

Check if a string contains a substring:

```json
{
  "functionCall": {
    "name": "contains",
    "args": [
      {
        "path": "/email"
      },
      "@"
    ]
  }
}
```

Returns `true` or `false`.

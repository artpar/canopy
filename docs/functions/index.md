---
layout: default
title: Functions
nav_order: 6
has_children: true
---

# Functions

Canopy includes 50+ built-in functions for string manipulation, math, logic, arrays, objects, and system operations. Functions are used in expressions to compute dynamic values.

## Syntax

Call a function with `functionCall`:

```json
{
  "functionCall": {
    "name": "concat",
    "args": [
      "Hello, ",
      {
        "path": "/name"
      }
    ]
  }
}
```

Arguments can be:
- **Literals**: strings, numbers, booleans
- **Path references**: `{"path": "/data/value"}`
- **Nested function calls**: `{"functionCall": {"name": "add", "args": [1, 2]}}`

## Categories

| Category | Functions |
|----------|-----------|
| [String](string) | concat, toString, toUpperCase, toLowerCase, trim, substring, substringAfter, replace, format, contains |
| [Math](math) | add, subtract, multiply, divide, calc, negate, toNumber |
| [Logic](logic) | equals, not, greaterThan, lessThan, if, or, and |
| [Arrays](arrays) | append, removeLast, slice, filter, filterContains, filterContainsAny, find, sort, remove, insertAt, length, countWhere |
| [Objects](objects) | getField, setField, updateItem |
| [Utility](utility) | toString, format, length, contains, now, uuid, formatDateRelative |
| [Trees](trees) | appendToTree, removeFromTree |

## Resolution Order

When a function name is called, Canopy looks it up in this order:

1. **Built-in functions** -- the functions documented in this section
2. **User-defined functions** -- registered via [defineFunction](../protocol/define-function)
3. **FFI functions** -- loaded via [loadLibrary](../protocol/load-library)

If multiple sources define the same name, the first match wins.

## Lazy Evaluation

Most functions resolve all arguments before executing. Three functions use **lazy evaluation** -- they only evaluate the arguments they need:

- **if** -- only evaluates the chosen branch
- **or** -- stops at the first truthy value
- **and** -- stops at the first falsy value

This prevents errors from evaluating branches that should not run.

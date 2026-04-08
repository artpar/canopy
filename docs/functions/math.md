---
layout: default
title: Math
parent: Functions
nav_order: 2
---

# Math Functions

Functions for arithmetic operations.

## Reference

| Function | Args | Returns | Description |
|----------|------|---------|-------------|
| `add` | `a, b` | number | Addition (a + b) |
| `subtract` | `a, b` | number | Subtraction (a - b) |
| `multiply` | `a, b` | number | Multiplication (a * b) |
| `divide` | `a, b` | number | Division (a / b) |
| `calc` | `op, left, right` | number | Dynamic operator: `"+"`, `"-"`, `"*"`, or `"/"` |
| `negate` | `n` | number | Negate a number (-n) |
| `toNumber` | `s` | number | Convert string to number |

## Examples

### Basic Arithmetic

```json
{
  "functionCall": {
    "name": "add",
    "args": [
      {
        "path": "/price"
      },
      {
        "path": "/tax"
      }
    ]
  }
}
```

```json
{
  "functionCall": {
    "name": "multiply",
    "args": [
      {
        "path": "/quantity"
      },
      {
        "path": "/unitPrice"
      }
    ]
  }
}
```

### calc

Perform an operation with a dynamic operator -- useful when the operation itself comes from the data model:

```json
{
  "functionCall": {
    "name": "calc",
    "args": [
      {
        "path": "/operator"
      },
      {
        "path": "/left"
      },
      {
        "path": "/right"
      }
    ]
  }
}
```

The first argument must be one of `"+"`, `"-"`, `"*"`, or `"/"`.

### Chained Arithmetic

Nest function calls for complex expressions:

```json
{
  "functionCall": {
    "name": "subtract",
    "args": [
      {
        "functionCall": {
          "name": "multiply",
          "args": [
            {
              "path": "/quantity"
            },
            {
              "path": "/price"
            }
          ]
        }
      },
      {
        "path": "/discount"
      }
    ]
  }
}
```

This computes `(quantity * price) - discount`.

### toNumber

Convert a string to a number for arithmetic:

```json
{
  "functionCall": {
    "name": "add",
    "args": [
      {
        "functionCall": {
          "name": "toNumber",
          "args": [
            {
              "path": "/inputValue"
            }
          ]
        }
      },
      1
    ]
  }
}
```

### negate

Flip the sign of a number:

```json
{
  "functionCall": {
    "name": "negate",
    "args": [
      {
        "path": "/balance"
      }
    ]
  }
}
```

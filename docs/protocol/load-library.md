---
layout: default
title: loadLibrary
parent: Protocol Reference
nav_order: 6
---

# loadLibrary

Dynamically loads a native C shared library and registers its functions for use in expressions.

## Example

```json
{
  "type": "loadLibrary",
  "path": "/usr/local/lib/libmath.dylib",
  "prefix": "math_",
  "functions": [
    {
      "name": "factorial",
      "symbol": "math_factorial",
      "returnType": "int",
      "paramTypes": [
        "int"
      ]
    },
    {
      "name": "fibonacci",
      "symbol": "math_fib",
      "returnType": "int",
      "paramTypes": [
        "int"
      ]
    }
  ]
}
```

## Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `type` | string | yes | `"loadLibrary"` |
| `path` | string | yes | Path to the shared library (.dylib) |
| `prefix` | string | no | Common prefix for symbol names |
| `functions` | array | yes | Functions to register |

### Function Definition

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | string | yes | Name to use in function calls |
| `symbol` | string | yes | Symbol name in the library |
| `returnType` | string | no | Return type: `"int"`, `"float"`, `"string"`, `"void"` |
| `paramTypes` | array | no | Array of parameter types |
| `fixedArgs` | int | no | Number of fixed arguments (for variadic functions) |

## Behavior

- Uses `dlopen` to load the library and `dlsym` to resolve symbols.
- Registered functions are callable from expressions: `{"functionCall": {"name": "factorial", "args": [5]}}`.
- FFI functions are checked after built-in and user-defined functions in the resolution order.
- Uses libffi for type-safe foreign function invocation.

## Related

- [defineFunction](define-function) -- register functions without FFI
- [Expressions guide](../guide/expressions) -- how to call functions in props

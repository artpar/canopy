---
layout: default
title: defineFunction
parent: Protocol Reference
nav_order: 7
---

# defineFunction

Registers a reusable parametric function that can be called from expressions and actions.

## Example

```json
{
  "type": "defineFunction",
  "name": "appendDigit",
  "params": [
    "digit"
  ],
  "body": {
    "functionCall": {
      "call": "updateDataModel",
      "args": {
        "ops": [
          {
            "op": "replace",
            "path": "/display",
            "value": {
              "functionCall": {
                "name": "concat",
                "args": [
                  {
                    "path": "/display"
                  },
                  {
                    "param": "digit"
                  }
                ]
              }
            }
          }
        ]
      }
    }
  }
}
```

## Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `type` | string | yes | `"defineFunction"` |
| `name` | string | yes | Function name (used in `functionCall`) |
| `params` | array | yes | Parameter names |
| `body` | any | yes | Function body with `{"param": "name"}` placeholders |

## Behavior

- The function is registered globally and available to all surfaces.
- When called, arguments are resolved first, then the body is deep-copied with `{"param": "name"}` placeholders replaced by resolved values.
- The resulting expression is evaluated.
- If a function with the same name as a built-in is defined, the built-in takes precedence.

## Calling a Defined Function

In an expression:
```json
{
  "functionCall": {
    "name": "appendDigit",
    "args": [
      "5"
    ]
  }
}
```

In an action:
```json
{
  "action": {
    "functionCall": {
      "call": "appendDigit",
      "args": [
        "5"
      ]
    }
  }
}
```

## Related

- [Reusable Functions guide](../guide/reusable-functions) -- detailed examples
- [defineComponent](define-component) -- reusable component templates

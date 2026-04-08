---
layout: default
title: sendToProcess
parent: Protocol Reference
nav_order: 12
---

# sendToProcess

Routes a message to a running process's transport.

## Example

```json
{
  "type": "sendToProcess",
  "processId": "agent",
  "message": {
    "type": "updateDataModel",
    "surfaceId": "main",
    "ops": [
      {
        "op": "replace",
        "path": "/prompt",
        "value": "Add a delete button"
      }
    ]
  }
}
```

## Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `type` | string | yes | `"sendToProcess"` |
| `processId` | string | yes | Target process ID |
| `message` | object | yes | Any valid A2UI message to send to the process |

## Behavior

- The message is forwarded to the process's transport.
- For LLM transports, this sends the message as context to the LLM.
- If the process does not exist or is not running, the message is ignored.

## Related

- [createProcess](create-process) -- start a process
- [stopProcess](stop-process) -- terminate a process

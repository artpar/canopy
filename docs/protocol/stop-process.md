---
layout: default
title: stopProcess
parent: Protocol Reference
nav_order: 11
---

# stopProcess

Terminates a running process.

## Example

```json
{
  "type": "stopProcess",
  "processId": "timer"
}
```

## Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `type` | string | yes | `"stopProcess"` |
| `processId` | string | yes | ID of the process to stop |

## Behavior

- Sends a cancel signal to the process goroutine.
- The process status at `/processes/{processId}/status` is updated to `"stopped"`.
- If the process does not exist, the message is ignored.
- The process's transport is stopped and cleaned up.

## Related

- [createProcess](create-process) -- start a process
- [sendToProcess](send-to-process) -- send a message to a process

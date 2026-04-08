---
layout: default
title: createProcess
parent: Protocol Reference
nav_order: 10
---

# createProcess

Spawns a background process with its own message transport.

## Example

```json
{
  "type": "createProcess",
  "processId": "timer",
  "transport": {
    "type": "interval",
    "interval": 1000,
    "message": {
      "type": "updateDataModel",
      "surfaceId": "main",
      "ops": [
        {
          "op": "replace",
          "path": "/tick",
          "value": {
            "functionCall": {
              "name": "add",
              "args": [
                {
                  "path": "/tick"
                },
                1
              ]
            }
          }
        }
      ]
    }
  }
}
```

## Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `type` | string | yes | `"createProcess"` |
| `processId` | string | yes | Unique process identifier |
| `transport` | object | yes | Transport configuration |

### Transport Configuration

| Field | Type | Used By | Description |
|-------|------|---------|-------------|
| `type` | string | all | `"file"`, `"interval"`, or `"llm"` |
| `path` | string | file | Path to JSONL file |
| `interval` | int | interval | Milliseconds between messages |
| `message` | object | interval | Message to send on each tick |
| `provider` | string | llm | LLM provider name |
| `model` | string | llm | Model identifier |
| `prompt` | string | llm | System prompt for the LLM |

## Behavior

- A new goroutine is started for the process.
- Process status is written to the data model at `/processes/{processId}/status`.
- Status values: `"running"`, `"stopped"`, `"error"`.
- Messages from the process are routed through the main session, so they can create surfaces, update components, etc.
- If a process with the same `processId` already exists, the message returns an error.

## Transport Types

### file
Reads a JSONL file and processes each message. The process stops when the file ends.

### interval
Sends the configured `message` every `interval` milliseconds. Runs until stopped.

### llm
Connects to an LLM provider and streams A2UI messages. The LLM receives the A2UI system prompt and can create surfaces, components, and handle events.

## Related

- [stopProcess](stop-process) -- terminate a process
- [sendToProcess](send-to-process) -- send a message to a running process
- [Processes guide](../guide/processes) -- examples and patterns

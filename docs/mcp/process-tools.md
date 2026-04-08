---
layout: default
title: Process Management
parent: MCP Tools
nav_order: 8
---

# Process Management

Canopy can spawn and manage child processes. Each process runs its own transport (file, LLM, or pipe) and has its own set of surfaces. Use these tools to create multi-process architectures where different parts of your app are driven by different sources.

---

## list_processes

List all running processes.

**Parameters:** None

**Returns:** Array of process objects with their IDs and status.

**Example:**
```
mcp__canopy__list_processes()
```

```json
[
  {
    "id": "main",
    "status": "running"
  },
  {
    "id": "worker-1",
    "status": "running"
  }
]
```

---

## create_process

Spawn a new child process with its own transport.

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `processId` | string | yes | Unique identifier for the process |
| `transport` | object | yes | Transport configuration (type, path, args, etc.) |

**Example -- file transport:**
```
mcp__canopy__create_process(
  processId: "sidebar",
  transport: { "type": "file", "path": "sidebar.jsonl" }
)
```

**Example -- command transport:**
```
mcp__canopy__create_process(
  processId: "agent",
  transport: { "type": "command", "cmd": "python", "args": ["agent.py"] }
)
```

The new process starts immediately and its messages create surfaces and components just like the main process.

---

## stop_process

Stop a running process and clean up its resources.

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `processId` | string | yes | The process to stop |

**Example:**
```
mcp__canopy__stop_process(processId: "worker-1")
```

---

## send_to_process

Send a message to a running process's input stream.

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `processId` | string | yes | The target process |
| `message` | string | yes | The message to send (JSONL string) |

**Example:**
```
mcp__canopy__send_to_process(
  processId: "agent",
  message: "{\"type\":\"actionResponse\",\"actionId\":\"act_1\",\"data\":{\"result\":\"ok\"}}"
)
```

This is how you feed responses back to LLM agents or other interactive processes.

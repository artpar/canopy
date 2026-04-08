---
layout: default
title: Channel Communication
parent: MCP Tools
nav_order: 9
---

# Channel Communication

Channels provide pub/sub messaging between processes. A channel is a named message bus that processes can publish to and subscribe from. Two modes are available: **broadcast** (all subscribers receive every message) and **queue** (messages are distributed round-robin to subscribers).

---

## list_channels

List all channels and their subscribers.

**Parameters:** None

**Returns:** Array of channel objects.

**Example:**
```
mcp__canopy__list_channels()
```

```json
[
  {
    "id": "updates",
    "mode": "broadcast",
    "subscribers": 3
  },
  {
    "id": "tasks",
    "mode": "queue",
    "subscribers": 2
  }
]
```

---

## create_channel

Create a new channel.

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `channelId` | string | yes | Unique channel name |
| `mode` | string | yes | `"broadcast"` or `"queue"` |

**Example:**
```
mcp__canopy__create_channel(channelId: "notifications", mode: "broadcast")
```

- **broadcast**: Every subscriber receives every published message.
- **queue**: Each message goes to exactly one subscriber, round-robin.

---

## delete_channel

Delete a channel and remove all its subscriptions.

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `channelId` | string | yes | The channel to delete |

**Example:**
```
mcp__canopy__delete_channel(channelId: "notifications")
```

---

## publish

Publish a value to a channel.

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `channelId` | string | yes | The channel to publish to |
| `value` | any | yes | The value to publish (string, number, object, array) |

**Example:**
```
mcp__canopy__publish(
  channelId: "notifications",
  value: { "type": "alert", "message": "New item added" }
)
```

---

## subscribe

Subscribe a process or data model path to a channel.

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `channelId` | string | yes | The channel to subscribe to |
| `processId` | string | no | Subscribe this process's input stream |
| `targetPath` | string | no | Write received values to this data model path |

Provide either `processId` or `targetPath` (or both).

**Example -- subscribe a process:**
```
mcp__canopy__subscribe(channelId: "tasks", processId: "worker-1")
```

**Example -- subscribe a data model path:**
```
mcp__canopy__subscribe(channelId: "notifications", targetPath: "/ui/latest-notification")
```

When a message is published to the channel, it is delivered to the process's input stream or written to the data model path, which triggers re-rendering of any bound components.

---

## unsubscribe

Remove a subscription from a channel.

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `channelId` | string | yes | The channel to unsubscribe from |
| `processId` | string | no | The process to unsubscribe |
| `targetPath` | string | no | The data model path to unsubscribe |

**Example:**
```
mcp__canopy__unsubscribe(channelId: "tasks", processId: "worker-1")
```

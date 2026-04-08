---
layout: default
title: subscribe
parent: Protocol Reference
nav_order: 15
---

# subscribe

Registers a subscription on a channel. When values are published, they are written to the specified data model path.

## Example

```json
{
  "type": "subscribe",
  "channelId": "notifications",
  "targetPath": "/lastNotification"
}
```

## Fields

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `type` | string | yes | -- | `"subscribe"` |
| `channelId` | string | yes | -- | Channel to subscribe to |
| `processId` | string | no | `""` | Subscribe on behalf of a process (empty = session-level) |
| `targetPath` | string | no | `""` | JSON Pointer path where received values are written |

## Behavior

- When a value is published to the channel, it is written to `targetPath` on every surface's data model.
- Components bound to that path re-render automatically.
- In broadcast mode, all subscribers receive each value.
- In queue mode, values are delivered round-robin to one subscriber at a time.

## Related

- [unsubscribe](unsubscribe) -- remove a subscription
- [publish](publish-message) -- send a value to the channel
- [createChannel](create-channel) -- create the channel first

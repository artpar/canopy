---
layout: default
title: createChannel
parent: Protocol Reference
nav_order: 13
---

# createChannel

Creates a named communication channel for publish/subscribe messaging.

## Example

```json
{
  "type": "createChannel",
  "channelId": "notifications",
  "mode": "broadcast"
}
```

## Fields

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `type` | string | yes | -- | `"createChannel"` |
| `channelId` | string | yes | -- | Unique channel identifier |
| `mode` | string | no | `"broadcast"` | `"broadcast"` or `"queue"` |

## Modes

### broadcast
Every subscriber receives every published value. Use for notifications and status updates.

### queue
Values are delivered round-robin to subscribers. Each value goes to exactly one subscriber. Use for work distribution.

## Behavior

- Channel status is written to `/channels/{channelId}/value` in the data model when values are published.
- If a channel with the same `channelId` already exists, the message returns an error.

## Related

- [subscribe](subscribe) -- listen for channel values
- [publish](publish-message) -- send a value to the channel
- [deleteChannel](delete-channel) -- remove the channel
- [Channels guide](../guide/channels) -- examples and patterns

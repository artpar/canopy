---
layout: default
title: deleteChannel
parent: Protocol Reference
nav_order: 14
---

# deleteChannel

Removes a channel and all its subscriptions.

## Example

```json
{
  "type": "deleteChannel",
  "channelId": "notifications"
}
```

## Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `type` | string | yes | `"deleteChannel"` |
| `channelId` | string | yes | ID of the channel to remove |

## Behavior

- All subscriptions on the channel are removed.
- If the channel does not exist, the message is ignored.

## Related

- [createChannel](create-channel) -- create a channel

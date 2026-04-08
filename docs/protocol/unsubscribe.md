---
layout: default
title: unsubscribe
parent: Protocol Reference
nav_order: 16
---

# unsubscribe

Removes a subscription from a channel.

## Example

Remove a specific subscription:
```json
{
  "type": "unsubscribe",
  "channelId": "notifications",
  "targetPath": "/lastNotification"
}
```

Remove all subscriptions for a process:
```json
{
  "type": "unsubscribe",
  "channelId": "notifications",
  "processId": "worker1"
}
```

## Fields

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `type` | string | yes | -- | `"unsubscribe"` |
| `channelId` | string | yes | -- | Channel to unsubscribe from |
| `processId` | string | no | `""` | Process whose subscriptions to remove |
| `targetPath` | string | no | `""` | Specific subscription to remove |

## Behavior

- If `targetPath` is set, only that specific subscription is removed.
- If `targetPath` is empty, all subscriptions for the given `processId` are removed.
- If the channel or subscription does not exist, the message is ignored.

## Related

- [subscribe](subscribe) -- add a subscription
- [deleteChannel](delete-channel) -- remove the entire channel

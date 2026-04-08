---
layout: default
title: publish
parent: Protocol Reference
nav_order: 17
---

# publish

Sends a value to a channel, delivering it to all subscribers (broadcast) or the next subscriber (queue).

## Example

```json
{
  "type": "publish",
  "channelId": "notifications",
  "value": "System update available"
}
```

With an object value:

```json
{
  "type": "publish",
  "channelId": "events",
  "value": {
    "type": "click",
    "target": "submit",
    "timestamp": 1234567890
  }
}
```

## Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `type` | string | yes | `"publish"` |
| `channelId` | string | yes | Target channel ID |
| `value` | any | yes | Value to publish (any JSON type) |

## Behavior

- The value is delivered to subscribers based on the channel's mode:
  - **broadcast**: every subscriber receives the value
  - **queue**: one subscriber receives it (round-robin)
- The value is written to each subscriber's `targetPath` in the data model.
- The last published value is stored at `/channels/{channelId}/value`.
- If the channel does not exist, the message is ignored.

## Related

- [subscribe](subscribe) -- register to receive values
- [createChannel](create-channel) -- create the channel

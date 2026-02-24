package protocol

// CreateChannel registers a named channel for inter-process communication.
type CreateChannel struct {
	Type       MessageType `json:"type"`
	ChannelID  string      `json:"channelId"`
	Mode       string      `json:"mode,omitempty"`       // "broadcast" (default) or "queue"
	BufferSize int         `json:"bufferSize,omitempty"` // reserved for future use
}

// DeleteChannel removes a channel and all its subscriptions.
type DeleteChannel struct {
	Type      MessageType `json:"type"`
	ChannelID string      `json:"channelId"`
}

// Publish sends a value to a channel.
type Publish struct {
	Type      MessageType `json:"type"`
	ChannelID string      `json:"channelId"`
	Value     interface{} `json:"value"`
}

// Subscribe registers interest in a channel's values.
type Subscribe struct {
	Type       MessageType `json:"type"`
	ChannelID  string      `json:"channelId"`
	ProcessID  string      `json:"processId,omitempty"`  // empty = session-level subscriber
	TargetPath string      `json:"targetPath,omitempty"` // DataModel path to deliver values
}

// Unsubscribe removes a subscription from a channel.
// If TargetPath is set, only that specific subscription is removed.
// If TargetPath is empty, all subscriptions for the processId are removed.
type Unsubscribe struct {
	Type       MessageType `json:"type"`
	ChannelID  string      `json:"channelId"`
	ProcessID  string      `json:"processId,omitempty"`  // empty = session-level subscriber
	TargetPath string      `json:"targetPath,omitempty"` // if set, only remove this specific subscription
}

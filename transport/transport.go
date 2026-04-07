package transport

import "canopy/protocol"

// Transport delivers A2UI messages to the engine.
type Transport interface {
	// Messages returns a channel of parsed messages.
	// The channel is closed when the transport is done.
	Messages() <-chan *protocol.Message

	// Errors returns a channel of transport errors.
	Errors() <-chan error

	// Start begins reading messages. Non-blocking.
	Start()

	// Stop terminates the transport.
	Stop()

	// SendAction sends a user event back to the source.
	// For file transport this is a no-op. For LLM transport it triggers a new conversation turn.
	SendAction(surfaceID string, event *protocol.EventDef, data map[string]interface{})
}

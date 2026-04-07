package engine

import (
	"fmt"
	"canopy/protocol"
	"sync"
)

// ChannelMode determines how published values are delivered to subscribers.
type ChannelMode string

const (
	ChannelBroadcast ChannelMode = "broadcast"
	ChannelQueue     ChannelMode = "queue"
)

// Subscription represents a single subscriber on a channel.
type Subscription struct {
	ProcessID  string // empty = session-level
	TargetPath string // DataModel path to write received values
}

// Channel is a named communication primitive.
type Channel struct {
	ID          string
	Mode        ChannelMode
	Subscribers []Subscription
	LastValue   interface{}
	roundRobin  int // queue mode: next consumer index
}

// ChannelManager manages named channels for inter-process communication.
type ChannelManager struct {
	mu       sync.Mutex
	channels map[string]*Channel
	sess     *Session
}

// NewChannelManager creates a ChannelManager attached to the given session.
func NewChannelManager(sess *Session) *ChannelManager {
	return &ChannelManager{
		channels: make(map[string]*Channel),
		sess:     sess,
	}
}

// Create registers a new channel.
func (cm *ChannelManager) Create(cc protocol.CreateChannel) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	if _, exists := cm.channels[cc.ChannelID]; exists {
		return fmt.Errorf("channel %q already exists", cc.ChannelID)
	}

	mode := ChannelBroadcast
	switch cc.Mode {
	case "queue":
		mode = ChannelQueue
	case "", "broadcast":
		// default
	default:
		logWarn("channel", "", fmt.Sprintf("unknown channel mode %q for %q, defaulting to broadcast", cc.Mode, cc.ChannelID))
	}

	cm.channels[cc.ChannelID] = &Channel{
		ID:   cc.ChannelID,
		Mode: mode,
	}

	cm.setStatusLocked(cc.ChannelID, "active")
	return nil
}

// Delete removes a channel and all its subscriptions.
func (cm *ChannelManager) Delete(channelID string) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	if _, exists := cm.channels[channelID]; !exists {
		return fmt.Errorf("channel %q not found", channelID)
	}

	delete(cm.channels, channelID)
	cm.setStatusLocked(channelID, "deleted")
	return nil
}

// Subscribe adds a subscriber to a channel.
func (cm *ChannelManager) Subscribe(sub protocol.Subscribe) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	ch, exists := cm.channels[sub.ChannelID]
	if !exists {
		return fmt.Errorf("channel %q not found", sub.ChannelID)
	}

	// Dedup by processId + targetPath
	for _, s := range ch.Subscribers {
		if s.ProcessID == sub.ProcessID && s.TargetPath == sub.TargetPath {
			return nil // already subscribed
		}
	}

	ch.Subscribers = append(ch.Subscribers, Subscription{
		ProcessID:  sub.ProcessID,
		TargetPath: sub.TargetPath,
	})
	return nil
}

// Unsubscribe removes a subscriber from a channel.
func (cm *ChannelManager) Unsubscribe(unsub protocol.Unsubscribe) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	ch, exists := cm.channels[unsub.ChannelID]
	if !exists {
		return fmt.Errorf("channel %q not found", unsub.ChannelID)
	}

	filtered := make([]Subscription, 0, len(ch.Subscribers))
	for _, s := range ch.Subscribers {
		match := s.ProcessID == unsub.ProcessID
		if match && unsub.TargetPath != "" {
			match = s.TargetPath == unsub.TargetPath
		}
		if !match {
			filtered = append(filtered, s)
		}
	}
	ch.Subscribers = filtered
	return nil
}

// Publish sends a value to a channel. The value is written to /channels/{id}/value
// on all surfaces. Subscribers receive the value at their targetPath.
func (cm *ChannelManager) Publish(pub protocol.Publish) error {
	cm.mu.Lock()

	ch, exists := cm.channels[pub.ChannelID]
	if !exists {
		cm.mu.Unlock()
		return fmt.Errorf("channel %q not found", pub.ChannelID)
	}

	ch.LastValue = pub.Value

	// Copy subscribers and mode under lock
	subs := make([]Subscription, len(ch.Subscribers))
	copy(subs, ch.Subscribers)
	mode := ch.Mode
	rr := ch.roundRobin

	// For queue mode, advance round-robin
	if mode == ChannelQueue && len(subs) > 0 {
		ch.roundRobin = (rr + 1) % len(subs)
	}

	cm.mu.Unlock()

	// Write to /channels/{id}/value on all surfaces
	valuePath := fmt.Sprintf("/channels/%s/value", pub.ChannelID)
	cm.sess.forEachSurfaceLocked(func(sid string, surf *Surface) {
		surf.HandleUpdateDataModel(protocol.UpdateDataModel{
			SurfaceID: sid,
			Ops:       []protocol.DataModelOp{{Op: "replace", Path: valuePath, Value: pub.Value}},
		})
	})

	// Deliver to subscribers
	switch mode {
	case ChannelBroadcast:
		for _, s := range subs {
			if s.TargetPath != "" {
				cm.deliverToPath(s.TargetPath, pub.Value)
			}
		}
	case ChannelQueue:
		if len(subs) > 0 {
			s := subs[rr%len(subs)]
			if s.TargetPath != "" {
				cm.deliverToPath(s.TargetPath, pub.Value)
			}
		}
	}

	return nil
}

// IDs returns the IDs of all channels.
func (cm *ChannelManager) IDs() []string {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	ids := make([]string, 0, len(cm.channels))
	for id := range cm.channels {
		ids = append(ids, id)
	}
	return ids
}

// GetChannel returns channel info for inspection, or nil if not found.
func (cm *ChannelManager) GetChannel(id string) *Channel {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	return cm.channels[id]
}

// CleanupProcess removes all subscriptions belonging to the given processId.
func (cm *ChannelManager) CleanupProcess(processID string) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	for _, ch := range cm.channels {
		filtered := ch.Subscribers[:0]
		for _, s := range ch.Subscribers {
			if s.ProcessID != processID {
				filtered = append(filtered, s)
			}
		}
		ch.Subscribers = filtered
	}
}

// setStatusLocked writes channel status to all surfaces.
func (cm *ChannelManager) setStatusLocked(channelID, status string) {
	path := fmt.Sprintf("/channels/%s/status", channelID)
	cm.sess.forEachSurfaceLocked(func(sid string, surf *Surface) {
		surf.HandleUpdateDataModel(protocol.UpdateDataModel{
			SurfaceID: sid,
			Ops:       []protocol.DataModelOp{{Op: "replace", Path: path, Value: status}},
		})
	})
}

// deliverToPath writes a value to a DataModel path on all surfaces.
func (cm *ChannelManager) deliverToPath(targetPath string, value interface{}) {
	cm.sess.forEachSurfaceLocked(func(sid string, surf *Surface) {
		surf.HandleUpdateDataModel(protocol.UpdateDataModel{
			SurfaceID: sid,
			Ops:       []protocol.DataModelOp{{Op: "replace", Path: targetPath, Value: value}},
		})
	})
}

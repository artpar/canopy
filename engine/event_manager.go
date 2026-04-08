package engine

import (
	"canopy/jlog"
	"canopy/protocol"
	"fmt"
	"sync"
)

// EventSubscription represents a single event subscription from an "on" message.
type EventSubscription struct {
	ID        string
	Event     string
	SurfaceID string
	Handler   protocol.EventAction
	Cancel    func() // stops timer, watcher, etc. Nil for passive subscriptions.
}

// EventManager manages event subscriptions from "on"/"off" protocol messages.
// It handles subscription lifecycle and cleanup.
type EventManager struct {
	mu   sync.Mutex
	subs map[string]*EventSubscription // subscriptionID → subscription
	sess *Session
	seq  int // auto-increment for unnamed subscriptions
}

// NewEventManager creates an EventManager attached to the given session.
func NewEventManager(sess *Session) *EventManager {
	return &EventManager{
		subs: make(map[string]*EventSubscription),
		sess: sess,
	}
}

// Subscribe registers an event subscription. If no ID is provided, one is generated.
func (em *EventManager) Subscribe(msg protocol.OnMessage) error {
	em.mu.Lock()
	defer em.mu.Unlock()

	id := msg.ID
	if id == "" {
		em.seq++
		id = fmt.Sprintf("__auto_%d", em.seq)
	}

	// Remove existing subscription with same ID
	if old, exists := em.subs[id]; exists {
		if old.Cancel != nil {
			old.Cancel()
		}
		delete(em.subs, id)
	}

	sub := &EventSubscription{
		ID:        id,
		Event:     msg.Event,
		SurfaceID: msg.SurfaceID,
		Handler:   msg.Handler,
	}

	em.subs[id] = sub

	jlog.Infof("events", msg.SurfaceID, "subscribed: id=%s event=%s", id, msg.Event)
	return nil
}

// Unsubscribe removes an event subscription by ID.
func (em *EventManager) Unsubscribe(id string) error {
	em.mu.Lock()
	defer em.mu.Unlock()

	sub, exists := em.subs[id]
	if !exists {
		return fmt.Errorf("subscription %q not found", id)
	}
	if sub.Cancel != nil {
		sub.Cancel()
	}
	delete(em.subs, id)
	jlog.Infof("events", "", "unsubscribed: id=%s", id)
	return nil
}

// Fire invokes all subscriptions matching the given event name and optional surfaceID.
// Called by native event sources when an event occurs.
func (em *EventManager) Fire(event string, surfaceID string, data string) {
	em.mu.Lock()
	// Collect matching subscriptions while holding the lock
	var matches []*EventSubscription
	for _, sub := range em.subs {
		if sub.Event != event {
			continue
		}
		if sub.SurfaceID != "" && sub.SurfaceID != surfaceID {
			continue
		}
		matches = append(matches, sub)
	}
	em.mu.Unlock()

	// Execute handlers outside the lock
	for _, sub := range matches {
		sid := sub.SurfaceID
		if sid == "" {
			sid = surfaceID
		}
		em.sess.mu.Lock()
		if surf, ok := em.sess.surfaces[sid]; ok {
			surf.executeEventAction(&sub.Handler, data)
		}
		em.sess.mu.Unlock()
	}
}

// CleanupSurface removes all subscriptions scoped to the given surface.
func (em *EventManager) CleanupSurface(surfaceID string) {
	em.mu.Lock()
	defer em.mu.Unlock()

	for id, sub := range em.subs {
		if sub.SurfaceID == surfaceID {
			if sub.Cancel != nil {
				sub.Cancel()
			}
			delete(em.subs, id)
		}
	}
}

// CleanupAll removes all subscriptions.
func (em *EventManager) CleanupAll() {
	em.mu.Lock()
	defer em.mu.Unlock()

	for id, sub := range em.subs {
		if sub.Cancel != nil {
			sub.Cancel()
		}
		delete(em.subs, id)
	}
}

package engine

import (
	"canopy/jlog"
	"canopy/protocol"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

// EventSubscription represents a single event subscription from an "on" message.
type EventSubscription struct {
	ID        string
	Event     string
	SurfaceID string
	Handler   protocol.EventAction
	Cancel    func()      // stops timer, watcher, etc. Nil for passive subscriptions.
	throttler *Throttler  // rate limiter, nil if no throttle/debounce configured
}

// SystemEventControl is called when the EventManager needs to start or stop
// a platform-specific event source. action is "start" or "stop".
type SystemEventControl func(event string, action string, config map[string]interface{})

// EventManager manages event subscriptions from "on"/"off" protocol messages.
// It handles subscription lifecycle and cleanup.
type EventManager struct {
	mu      sync.Mutex
	subs    map[string]*EventSubscription // subscriptionID → subscription
	sess    *Session
	seq     int // auto-increment for unnamed subscriptions
	control SystemEventControl
}

// NewEventManager creates an EventManager attached to the given session.
func NewEventManager(sess *Session) *EventManager {
	return &EventManager{
		subs: make(map[string]*EventSubscription),
		sess: sess,
	}
}

// SetControl sets the callback for starting/stopping platform event sources.
func (em *EventManager) SetControl(fn SystemEventControl) {
	em.mu.Lock()
	em.control = fn
	em.mu.Unlock()
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

	// Set up throttle/debounce if configured
	if msg.Handler.Throttle > 0 {
		sub.throttler = NewThrottler(msg.Handler.Throttle, "throttle")
	} else if msg.Handler.Debounce > 0 {
		sub.throttler = NewThrottler(msg.Handler.Debounce, "debounce")
	}

	// Start system event source if applicable
	em.startEventSource(sub, msg.Config)

	em.subs[id] = sub

	jlog.Infof("events", msg.SurfaceID, "subscribed: id=%s event=%s", id, msg.Event)
	return nil
}

// startEventSource starts the background process for system events that need active monitoring.
// Sets sub.Cancel to stop the source when the subscription is removed.
func (em *EventManager) startEventSource(sub *EventSubscription, config map[string]interface{}) {
	if !strings.HasPrefix(sub.Event, "system.") {
		return
	}

	switch sub.Event {
	case "system.timer":
		em.startTimer(sub, config)
	case "system.fs.watch":
		em.startFSWatch(sub, config)
	case "system.bluetooth", "system.location", "system.usb",
		"system.sensor.battery", "system.sensor.memory", "system.sensor.cpu",
		"system.sensor.disk", "system.sensor.uptime",
		"system.sensor.network.throughput", "system.sensor.audio",
		"system.sensor.display", "system.sensor.activeApp":
		em.startOnDemandSource(sub, config)
	case "system.ipc.distributed":
		em.startDistributedNotification(sub, config)
	case "system.network.websocket":
		em.startWebSocket(sub, config)
	case "system.network.sse":
		em.startSSE(sub, config)
	case "system.network.tcp":
		em.startTCPListener(sub, config)
	case "system.network.http":
		em.startHTTPListener(sub, config)
	}
}

// startTimer starts a periodic timer that fires the subscription's event.
func (em *EventManager) startTimer(sub *EventSubscription, config map[string]interface{}) {
	intervalMs := 1000 // default 1 second
	if config != nil {
		if v, ok := config["interval"]; ok {
			switch iv := v.(type) {
			case float64:
				intervalMs = int(iv)
			case json.Number:
				if n, err := iv.Int64(); err == nil {
					intervalMs = int(n)
				}
			}
		}
	}
	if intervalMs < 10 {
		intervalMs = 10 // minimum 10ms to prevent spin
	}

	done := make(chan struct{})
	sub.Cancel = func() { close(done) }

	surfaceID := sub.SurfaceID
	event := sub.Event

	go func() {
		ticker := time.NewTicker(time.Duration(intervalMs) * time.Millisecond)
		defer ticker.Stop()
		tick := 0
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				tick++
				data := fmt.Sprintf(`{"tick":%d,"elapsed":%d}`, tick, tick*intervalMs)
				em.Fire(event, surfaceID, data)
			}
		}
	}()
}

// startFSWatch starts a filesystem watcher for the given paths.
func (em *EventManager) startFSWatch(sub *EventSubscription, config map[string]interface{}) {
	var paths []string
	if config != nil {
		if v, ok := config["paths"]; ok {
			switch pv := v.(type) {
			case []interface{}:
				for _, p := range pv {
					if s, ok := p.(string); ok {
						paths = append(paths, s)
					}
				}
			case string:
				paths = append(paths, pv)
			}
		}
		if v, ok := config["path"]; ok {
			if s, ok := v.(string); ok {
				paths = append(paths, s)
			}
		}
	}
	if len(paths) == 0 {
		jlog.Errorf("events", "", "system.fs.watch: no paths specified")
		return
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		jlog.Errorf("events", "", "system.fs.watch: failed to create watcher: %v", err)
		return
	}

	for _, p := range paths {
		if err := watcher.Add(p); err != nil {
			jlog.Errorf("events", "", "system.fs.watch: failed to watch %q: %v", p, err)
		}
	}

	done := make(chan struct{})
	sub.Cancel = func() {
		close(done)
		watcher.Close()
	}

	surfaceID := sub.SurfaceID
	event := sub.Event

	go func() {
		for {
			select {
			case <-done:
				return
			case ev, ok := <-watcher.Events:
				if !ok {
					return
				}
				op := "unknown"
				switch {
				case ev.Has(fsnotify.Create):
					op = "created"
				case ev.Has(fsnotify.Write):
					op = "modified"
				case ev.Has(fsnotify.Remove):
					op = "removed"
				case ev.Has(fsnotify.Rename):
					op = "renamed"
				case ev.Has(fsnotify.Chmod):
					op = "chmod"
				}
				data := fmt.Sprintf(`{"path":%q,"event":%q}`, ev.Name, op)
				em.Fire(event, surfaceID, data)
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				jlog.Errorf("events", "", "system.fs.watch error: %v", err)
			}
		}
	}()
}

// onDemandEvents tracks which on-demand sources need ref-counted start/stop.
var onDemandEvents = map[string]bool{
	"system.bluetooth":                true,
	"system.location":                 true,
	"system.usb":                      true,
	"system.sensor.battery":           true,
	"system.sensor.memory":            true,
	"system.sensor.cpu":               true,
	"system.sensor.disk":              true,
	"system.sensor.uptime":            true,
	"system.sensor.network.throughput": true,
	"system.sensor.audio":             true,
	"system.sensor.display":           true,
	"system.sensor.activeApp":         true,
}

// startOnDemandSource starts a platform event source via the control callback.
// Reference-counted: started on first subscription, stopped on last removal.
// Called with em.mu held.
func (em *EventManager) startOnDemandSource(sub *EventSubscription, config map[string]interface{}) {
	event := sub.Event

	// Count existing subscriptions for this event
	count := 0
	for _, s := range em.subs {
		if s.Event == event {
			count++
		}
	}

	// First subscriber: start the source
	if count == 0 && em.control != nil {
		em.control(event, "start", config)
	}
	// No Cancel needed — stopOnDemandIfEmpty handles cleanup
}

// stopOnDemandIfEmpty stops an on-demand source if no subscribers remain.
// Called with em.mu held.
func (em *EventManager) stopOnDemandIfEmpty(event string) {
	if !onDemandEvents[event] || em.control == nil {
		return
	}
	for _, s := range em.subs {
		if s.Event == event {
			return // still has subscribers
		}
	}
	em.control(event, "stop", nil)
}

// startDistributedNotification starts observing a named distributed notification.
func (em *EventManager) startDistributedNotification(sub *EventSubscription, config map[string]interface{}) {
	name := ""
	if config != nil {
		if v, ok := config["name"]; ok {
			name, _ = v.(string)
		}
	}
	if name == "" {
		jlog.Errorf("events", "", "system.ipc.distributed: no notification name specified")
		return
	}

	if em.control != nil {
		em.control("system.ipc.distributed", "start", map[string]interface{}{
			"name":           name,
			"subscriptionID": sub.ID,
		})
	}

	sub.Cancel = func() {
		if em.control != nil {
			em.control("system.ipc.distributed", "stop", map[string]interface{}{
				"subscriptionID": sub.ID,
			})
		}
	}
}

// Unsubscribe removes an event subscription by ID.
func (em *EventManager) Unsubscribe(id string) error {
	em.mu.Lock()
	defer em.mu.Unlock()

	sub, exists := em.subs[id]
	if !exists {
		return fmt.Errorf("subscription %q not found", id)
	}
	event := sub.Event
	if sub.throttler != nil {
		sub.throttler.Stop()
	}
	if sub.Cancel != nil {
		sub.Cancel()
	}
	delete(em.subs, id)
	em.stopOnDemandIfEmpty(event)
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
		execute := func() {
			em.sess.mu.Lock()
			if surf, ok := em.sess.surfaces[sid]; ok {
				surf.executeEventAction(&sub.Handler, data)
			}
			em.sess.mu.Unlock()
		}
		if sub.throttler != nil {
			sub.throttler.Call(execute)
		} else {
			execute()
		}
	}
}

// CleanupSurface removes all subscriptions scoped to the given surface.
func (em *EventManager) CleanupSurface(surfaceID string) {
	em.mu.Lock()
	defer em.mu.Unlock()

	var removedEvents []string
	for id, sub := range em.subs {
		if sub.SurfaceID == surfaceID {
			if sub.throttler != nil {
				sub.throttler.Stop()
			}
			if sub.Cancel != nil {
				sub.Cancel()
			}
			removedEvents = append(removedEvents, sub.Event)
			delete(em.subs, id)
		}
	}
	for _, event := range removedEvents {
		em.stopOnDemandIfEmpty(event)
	}
}

// CleanupAll removes all subscriptions.
func (em *EventManager) CleanupAll() {
	em.mu.Lock()
	defer em.mu.Unlock()

	var removedEvents []string
	for id, sub := range em.subs {
		if sub.throttler != nil {
			sub.throttler.Stop()
		}
		if sub.Cancel != nil {
			sub.Cancel()
		}
		removedEvents = append(removedEvents, sub.Event)
		delete(em.subs, id)
	}
	for _, event := range removedEvents {
		em.stopOnDemandIfEmpty(event)
	}
}

package darwin

import (
	"jview/jlog"
	"runtime"
	"sync"
)

// CallbackRegistry maps uint64 IDs to Go callback functions.
// Used by ObjC target-action to route events back to Go.
type CallbackRegistry struct {
	mu      sync.RWMutex
	next    uint64
	entries map[uint64]func(string)
}

var globalRegistry = &CallbackRegistry{
	next:    1,
	entries: make(map[uint64]func(string)),
}

// Register stores a callback and returns its ID.
func (r *CallbackRegistry) Register(fn func(string)) uint64 {
	r.mu.Lock()
	defer r.mu.Unlock()
	id := r.next
	r.next++
	r.entries[id] = fn
	return id
}

// Invoke calls the callback with the given ID.
func (r *CallbackRegistry) Invoke(id uint64, data string) {
	r.mu.RLock()
	fn, ok := r.entries[id]
	r.mu.RUnlock()
	if ok {
		defer func() {
			if rec := recover(); rec != nil {
				buf := make([]byte, 4096)
				n := runtime.Stack(buf, false)
				jlog.Errorf("darwin", "", "callback/%d panic recovered: %v\n%s", id, rec, buf[:n])
			}
		}()
		fn(data)
	}
}

// Update replaces the function for an existing ID, re-adding it if previously unregistered.
func (r *CallbackRegistry) Update(id uint64, fn func(string)) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.entries[id] = fn
}

// Unregister removes a callback.
func (r *CallbackRegistry) Unregister(id uint64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.entries, id)
}

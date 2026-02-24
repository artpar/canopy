package engine

import (
	"jview/protocol"
	"jview/renderer"
	"testing"
)

func setupChannelTest(t *testing.T) (*Session, *ChannelManager) {
	t.Helper()
	mock := renderer.NewMockRenderer()
	disp := &renderer.MockDispatcher{}
	sess := NewSession(mock, disp)

	// Create a surface so DataModel writes have somewhere to go
	feedMessages(t, sess, `{"type":"createSurface","surfaceId":"main","title":"Test"}`)

	cm := NewChannelManager(sess)
	sess.SetChannelManager(cm)
	return sess, cm
}

func TestChannelCreateAndDelete(t *testing.T) {
	_, cm := setupChannelTest(t)

	// Create a channel
	err := cm.Create(protocol.CreateChannel{ChannelID: "notif", Mode: "broadcast"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	// Duplicate should error
	err = cm.Create(protocol.CreateChannel{ChannelID: "notif"})
	if err == nil {
		t.Fatal("expected duplicate error")
	}

	// Channel exists
	ch := cm.GetChannel("notif")
	if ch == nil {
		t.Fatal("channel not found after create")
	}
	if ch.Mode != ChannelBroadcast {
		t.Errorf("mode = %q, want broadcast", ch.Mode)
	}

	// Delete
	err = cm.Delete("notif")
	if err != nil {
		t.Fatalf("delete: %v", err)
	}

	// Gone
	if cm.GetChannel("notif") != nil {
		t.Fatal("channel still exists after delete")
	}

	// Delete nonexistent should error
	err = cm.Delete("notif")
	if err == nil {
		t.Fatal("expected not-found error")
	}
}

func TestChannelPublishBroadcast(t *testing.T) {
	sess, cm := setupChannelTest(t)

	cm.Create(protocol.CreateChannel{ChannelID: "events", Mode: "broadcast"})

	// Subscribe two subscribers with different target paths
	cm.Subscribe(protocol.Subscribe{ChannelID: "events", TargetPath: "/sub1/value"})
	cm.Subscribe(protocol.Subscribe{ChannelID: "events", ProcessID: "p1", TargetPath: "/sub2/value"})

	// Publish a value
	err := cm.Publish(protocol.Publish{ChannelID: "events", Value: "hello"})
	if err != nil {
		t.Fatalf("publish: %v", err)
	}

	surf := sess.GetSurface("main")

	// /channels/events/value should be set
	val, ok := surf.DM().Get("/channels/events/value")
	if !ok {
		t.Fatal("channel value not in data model")
	}
	if val != "hello" {
		t.Errorf("channel value = %v, want hello", val)
	}

	// Both subscribers' target paths should be set
	val1, ok := surf.DM().Get("/sub1/value")
	if !ok {
		t.Fatal("/sub1/value not set")
	}
	if val1 != "hello" {
		t.Errorf("/sub1/value = %v, want hello", val1)
	}

	val2, ok := surf.DM().Get("/sub2/value")
	if !ok {
		t.Fatal("/sub2/value not set")
	}
	if val2 != "hello" {
		t.Errorf("/sub2/value = %v, want hello", val2)
	}
}

func TestChannelPublishQueue(t *testing.T) {
	sess, cm := setupChannelTest(t)

	cm.Create(protocol.CreateChannel{ChannelID: "work", Mode: "queue"})
	cm.Subscribe(protocol.Subscribe{ChannelID: "work", ProcessID: "w1", TargetPath: "/worker1/task"})
	cm.Subscribe(protocol.Subscribe{ChannelID: "work", ProcessID: "w2", TargetPath: "/worker2/task"})

	surf := sess.GetSurface("main")

	// First publish goes to worker1 (round-robin index 0)
	cm.Publish(protocol.Publish{ChannelID: "work", Value: "task-a"})
	val, _ := surf.DM().Get("/worker1/task")
	if val != "task-a" {
		t.Errorf("worker1 got %v, want task-a", val)
	}
	// worker2 should not have received it
	_, ok := surf.DM().Get("/worker2/task")
	if ok {
		t.Error("worker2 should not have received first task")
	}

	// Second publish goes to worker2 (round-robin index 1)
	cm.Publish(protocol.Publish{ChannelID: "work", Value: "task-b"})
	val2, _ := surf.DM().Get("/worker2/task")
	if val2 != "task-b" {
		t.Errorf("worker2 got %v, want task-b", val2)
	}

	// Third publish wraps around to worker1 again
	cm.Publish(protocol.Publish{ChannelID: "work", Value: "task-c"})
	val3, _ := surf.DM().Get("/worker1/task")
	if val3 != "task-c" {
		t.Errorf("worker1 got %v after wrap-around, want task-c", val3)
	}
}

func TestChannelSubscribeDedup(t *testing.T) {
	_, cm := setupChannelTest(t)

	cm.Create(protocol.CreateChannel{ChannelID: "ch"})
	cm.Subscribe(protocol.Subscribe{ChannelID: "ch", TargetPath: "/a"})
	cm.Subscribe(protocol.Subscribe{ChannelID: "ch", TargetPath: "/a"}) // duplicate

	ch := cm.GetChannel("ch")
	if len(ch.Subscribers) != 1 {
		t.Errorf("subscribers = %d, want 1 (dedup)", len(ch.Subscribers))
	}
}

func TestChannelUnsubscribe(t *testing.T) {
	_, cm := setupChannelTest(t)

	cm.Create(protocol.CreateChannel{ChannelID: "ch"})
	cm.Subscribe(protocol.Subscribe{ChannelID: "ch", ProcessID: "p1", TargetPath: "/a"})
	cm.Subscribe(protocol.Subscribe{ChannelID: "ch", ProcessID: "p2", TargetPath: "/b"})

	cm.Unsubscribe(protocol.Unsubscribe{ChannelID: "ch", ProcessID: "p1"})

	ch := cm.GetChannel("ch")
	if len(ch.Subscribers) != 1 {
		t.Errorf("subscribers = %d, want 1", len(ch.Subscribers))
	}
	if ch.Subscribers[0].ProcessID != "p2" {
		t.Errorf("remaining subscriber = %q, want p2", ch.Subscribers[0].ProcessID)
	}
}

func TestChannelCleanupProcess(t *testing.T) {
	_, cm := setupChannelTest(t)

	cm.Create(protocol.CreateChannel{ChannelID: "ch1"})
	cm.Create(protocol.CreateChannel{ChannelID: "ch2"})
	cm.Subscribe(protocol.Subscribe{ChannelID: "ch1", ProcessID: "p1", TargetPath: "/a"})
	cm.Subscribe(protocol.Subscribe{ChannelID: "ch1", ProcessID: "p2", TargetPath: "/b"})
	cm.Subscribe(protocol.Subscribe{ChannelID: "ch2", ProcessID: "p1", TargetPath: "/c"})

	cm.CleanupProcess("p1")

	ch1 := cm.GetChannel("ch1")
	if len(ch1.Subscribers) != 1 {
		t.Errorf("ch1 subscribers = %d, want 1", len(ch1.Subscribers))
	}

	ch2 := cm.GetChannel("ch2")
	if len(ch2.Subscribers) != 0 {
		t.Errorf("ch2 subscribers = %d, want 0", len(ch2.Subscribers))
	}
}

func TestChannelIDs(t *testing.T) {
	_, cm := setupChannelTest(t)

	cm.Create(protocol.CreateChannel{ChannelID: "a"})
	cm.Create(protocol.CreateChannel{ChannelID: "b"})

	ids := cm.IDs()
	if len(ids) != 2 {
		t.Fatalf("ids = %d, want 2", len(ids))
	}

	found := map[string]bool{}
	for _, id := range ids {
		found[id] = true
	}
	if !found["a"] || !found["b"] {
		t.Errorf("ids = %v, want [a, b]", ids)
	}
}

func TestChannelDataModelPath(t *testing.T) {
	sess, cm := setupChannelTest(t)

	cm.Create(protocol.CreateChannel{ChannelID: "status"})
	cm.Publish(protocol.Publish{ChannelID: "status", Value: map[string]interface{}{"code": 200}})

	surf := sess.GetSurface("main")
	val, ok := surf.DM().Get("/channels/status/value")
	if !ok {
		t.Fatal("channel value path not set")
	}
	m, ok := val.(map[string]interface{})
	if !ok {
		t.Fatalf("value type = %T, want map", val)
	}
	// Value goes through Go directly (not JSON), so code stays as int
	if m["code"] != 200 {
		t.Errorf("code = %v (%T), want 200", m["code"], m["code"])
	}
}

func TestChannelPublishToNonexistent(t *testing.T) {
	_, cm := setupChannelTest(t)

	err := cm.Publish(protocol.Publish{ChannelID: "nope", Value: "x"})
	if err == nil {
		t.Fatal("expected error publishing to nonexistent channel")
	}
}

func TestChannelSubscribeToNonexistent(t *testing.T) {
	_, cm := setupChannelTest(t)

	err := cm.Subscribe(protocol.Subscribe{ChannelID: "nope", TargetPath: "/a"})
	if err == nil {
		t.Fatal("expected error subscribing to nonexistent channel")
	}
}

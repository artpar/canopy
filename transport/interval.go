package transport

import (
	"encoding/json"
	"canopy/jlog"
	"canopy/protocol"
	"time"
)

// IntervalTransport sends a fixed message on a timer.
type IntervalTransport struct {
	interval time.Duration
	rawMsg   json.RawMessage
	msgs     chan *protocol.Message
	errs     chan error
	stop     chan struct{}
}

func NewIntervalTransport(intervalMS int, rawMsg json.RawMessage) *IntervalTransport {
	return &IntervalTransport{
		interval: time.Duration(intervalMS) * time.Millisecond,
		rawMsg:   rawMsg,
		msgs:     make(chan *protocol.Message, 16),
		errs:     make(chan error, 4),
		stop:     make(chan struct{}),
	}
}

func (t *IntervalTransport) Messages() <-chan *protocol.Message {
	return t.msgs
}

func (t *IntervalTransport) Errors() <-chan error {
	return t.errs
}

func (t *IntervalTransport) Start() {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				jlog.Errorf("transport", "", "panic in interval transport: %v", r)
			}
		}()
		ticker := time.NewTicker(t.interval)
		defer ticker.Stop()
		defer close(t.msgs)
		defer close(t.errs)

		for {
			select {
			case <-t.stop:
				return
			case <-ticker.C:
				msg, err := protocol.ParseLine(t.rawMsg)
				if err != nil {
					t.errs <- err
					continue
				}
				t.msgs <- msg
			}
		}
	}()
}

func (t *IntervalTransport) Stop() {
	select {
	case <-t.stop:
	default:
		close(t.stop)
	}
}

func (t *IntervalTransport) SendAction(surfaceID string, event *protocol.EventDef, data map[string]interface{}) {
	// no-op for interval transport
}

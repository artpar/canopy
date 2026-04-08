package engine

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"

	"canopy/jlog"

	"github.com/gorilla/websocket"
)

// startWebSocket connects to a WebSocket URL and fires events on each message.
func (em *EventManager) startWebSocket(sub *EventSubscription, config map[string]interface{}) {
	url := ""
	if config != nil {
		url, _ = config["url"].(string)
	}
	if url == "" {
		jlog.Errorf("events", "", "system.network.websocket: no url specified")
		return
	}

	done := make(chan struct{})
	surfaceID := sub.SurfaceID
	event := sub.Event

	go func() {
		conn, _, err := websocket.DefaultDialer.Dial(url, nil)
		if err != nil {
			em.Fire(event, surfaceID, fmt.Sprintf(`{"error":%q}`, err.Error()))
			return
		}

		// Set up cancel to close connection
		go func() {
			<-done
			conn.Close()
		}()

		for {
			_, message, err := conn.ReadMessage()
			if err != nil {
				select {
				case <-done:
					return // clean shutdown
				default:
					em.Fire(event, surfaceID, fmt.Sprintf(`{"error":%q}`, err.Error()))
					return
				}
			}
			em.Fire(event, surfaceID, string(message))
		}
	}()

	sub.Cancel = func() { close(done) }
}

// startSSE connects to a Server-Sent Events URL and fires events on each data message.
func (em *EventManager) startSSE(sub *EventSubscription, config map[string]interface{}) {
	url := ""
	if config != nil {
		url, _ = config["url"].(string)
	}
	if url == "" {
		jlog.Errorf("events", "", "system.network.sse: no url specified")
		return
	}

	done := make(chan struct{})
	surfaceID := sub.SurfaceID
	event := sub.Event

	go func() {
		resp, err := http.Get(url)
		if err != nil {
			em.Fire(event, surfaceID, fmt.Sprintf(`{"error":%q}`, err.Error()))
			return
		}
		defer resp.Body.Close()

		// Close body when canceled
		go func() {
			<-done
			resp.Body.Close()
		}()

		scanner := bufio.NewScanner(resp.Body)
		var dataLines []string

		for scanner.Scan() {
			line := scanner.Text()

			if strings.HasPrefix(line, "data:") {
				dataLines = append(dataLines, strings.TrimPrefix(line, "data:"))
			} else if line == "" && len(dataLines) > 0 {
				// Blank line = event boundary per SSE spec
				data := strings.TrimSpace(strings.Join(dataLines, "\n"))
				dataLines = nil
				if data != "" {
					em.Fire(event, surfaceID, data)
				}
			}
			// Ignore event:, id:, retry: lines for now
		}
	}()

	sub.Cancel = func() { close(done) }
}

// startTCPListener listens on a TCP port and fires events for each line received.
func (em *EventManager) startTCPListener(sub *EventSubscription, config map[string]interface{}) {
	port := 0
	if config != nil {
		switch v := config["port"].(type) {
		case float64:
			port = int(v)
		case int:
			port = v
		}
	}
	if port == 0 {
		jlog.Errorf("events", "", "system.network.tcp: no port specified")
		return
	}

	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		jlog.Errorf("events", "", "system.network.tcp: listen error: %v", err)
		return
	}

	done := make(chan struct{})
	surfaceID := sub.SurfaceID
	event := sub.Event

	// Accept loop
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				select {
				case <-done:
					return
				default:
					jlog.Errorf("events", "", "system.network.tcp: accept error: %v", err)
					return
				}
			}
			// Per-connection reader
			go func(c net.Conn) {
				defer c.Close()
				scanner := bufio.NewScanner(c)
				for scanner.Scan() {
					select {
					case <-done:
						return
					default:
						em.Fire(event, surfaceID, scanner.Text())
					}
				}
			}(conn)
		}
	}()

	sub.Cancel = func() {
		close(done)
		listener.Close()
	}
}

// startHTTPListener starts an HTTP server on a port and fires events for each request body.
func (em *EventManager) startHTTPListener(sub *EventSubscription, config map[string]interface{}) {
	port := 0
	if config != nil {
		switch v := config["port"].(type) {
		case float64:
			port = int(v)
		case int:
			port = v
		}
	}
	if port == 0 {
		jlog.Errorf("events", "", "system.network.http: no port specified")
		return
	}

	surfaceID := sub.SurfaceID
	event := sub.Event

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		data := string(body)
		if data == "" {
			data = fmt.Sprintf(`{"method":%q,"path":%q}`, r.Method, r.URL.Path)
		}
		em.Fire(event, surfaceID, data)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: mux,
	}

	done := make(chan struct{})

	go func() {
		if err := server.ListenAndServe(); err != http.ErrServerClosed {
			jlog.Errorf("events", "", "system.network.http: server error: %v", err)
		}
	}()

	sub.Cancel = func() {
		close(done)
		server.Close()
	}
}

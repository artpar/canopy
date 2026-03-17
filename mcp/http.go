package mcp

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"jview/jlog"
	"net"
	"net/http"
	"sync"
)

// ListenHTTP starts an HTTP MCP server on the given address.
// Use "localhost:0" for an OS-assigned port.
// Returns the actual port, a cleanup function, and any error.
func (s *Server) ListenHTTP(addr string) (int, func(), error) {
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return 0, nil, fmt.Errorf("mcp http listen: %w", err)
	}

	port := listener.Addr().(*net.TCPAddr).Port

	var (
		sessionID   string
		sessionOnce sync.Once
	)

	mux := http.NewServeMux()
	mux.HandleFunc("/mcp", func(w http.ResponseWriter, r *http.Request) {
		jlog.Infof("mcp", "", "http: %s %s from %s", r.Method, r.URL.Path, r.RemoteAddr)

		if r.Method == http.MethodOptions {
			// Handle CORS preflight
			origin := r.Header.Get("Origin")
			if origin != "" {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
				w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Mcp-Session-Id")
			}
			w.WriteHeader(http.StatusNoContent)
			return
		}

		if r.Method != http.MethodPost {
			jlog.Warnf("mcp", "", "http: rejected %s (not POST)", r.Method)
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Validate Origin header for security (localhost only)
		origin := r.Header.Get("Origin")
		if origin != "" {
			w.Header().Set("Access-Control-Allow-Origin", origin)
		}

		body, err := io.ReadAll(r.Body)
		r.Body.Close()
		if err != nil {
			jlog.Errorf("mcp", "", "http: read body error: %v", err)
			writeJSONRPCError(w, nil, ParseError, "failed to read body: "+err.Error())
			return
		}

		var req Request
		if err := json.Unmarshal(body, &req); err != nil {
			jlog.Errorf("mcp", "", "http: JSON parse error: %v body=%q", err, string(body[:min(len(body), 200)]))
			writeJSONRPCError(w, nil, ParseError, "invalid JSON: "+err.Error())
			return
		}

		jlog.Infof("mcp", "", "http: method=%s id=%v", req.Method, req.ID)

		// Handle initialize: generate session ID
		if req.Method == MethodInitialize {
			sessionOnce.Do(func() {
				sessionID = generateSessionID()
				jlog.Infof("mcp", "", "http: new session %s", sessionID)
			})
		}

		// Validate session ID on non-initialize requests
		if req.Method != MethodInitialize && sessionID != "" {
			sid := r.Header.Get("Mcp-Session-Id")
			if sid != sessionID {
				jlog.Warnf("mcp", "", "http: session mismatch got=%q want=%q", sid, sessionID)
				http.Error(w, "invalid session", http.StatusBadRequest)
				return
			}
		}

		resp := s.handle(&req)

		// Set session header
		if sessionID != "" {
			w.Header().Set("Mcp-Session-Id", sessionID)
		}

		// Notification — no response expected
		if resp == nil {
			w.WriteHeader(http.StatusAccepted)
			return
		}

		resp.ID = req.ID
		resp.JSONRPC = "2.0"

		w.Header().Set("Content-Type", "application/json")
		data, err := json.Marshal(resp)
		if err != nil {
			jlog.Errorf("mcp", "", "http: marshal error: %v", err)
			writeJSONRPCError(w, req.ID, InternalError, "marshal error")
			return
		}
		w.Write(data)
	})

	srv := &http.Server{Handler: mux}
	go func() {
		if err := srv.Serve(listener); err != nil && err != http.ErrServerClosed {
			jlog.Errorf("mcp", "", "http server error: %v", err)
		}
	}()

	cleanup := func() {
		srv.Close()
	}

	jlog.Infof("mcp", "", "http: listening on localhost:%d", port)
	return port, cleanup, nil
}

func generateSessionID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

func writeJSONRPCError(w http.ResponseWriter, id any, code int, message string) {
	resp := Response{
		JSONRPC: "2.0",
		ID:      id,
		Error: &ResponseError{
			Code:    code,
			Message: message,
		},
	}
	w.Header().Set("Content-Type", "application/json")
	data, _ := json.Marshal(resp)
	w.Write(data)
}

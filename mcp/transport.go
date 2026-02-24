package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sync"
)

// StdioTransport implements MCP transport over stdin/stdout.
type StdioTransport struct {
	reader  *bufio.Reader
	writer  io.Writer
	writeMu sync.Mutex
}

// NewStdioTransport creates a new stdio transport.
func NewStdioTransport(reader io.Reader, writer io.Writer) *StdioTransport {
	return &StdioTransport{
		reader: bufio.NewReader(reader),
		writer: writer,
	}
}

// ReadMessage reads a JSON-RPC message from the reader.
func (t *StdioTransport) ReadMessage() (*Request, error) {
	line, err := t.reader.ReadBytes('\n')
	if err != nil {
		return nil, err
	}

	var req Request
	if err := json.Unmarshal(line, &req); err != nil {
		return nil, fmt.Errorf("failed to parse JSON-RPC message: %w", err)
	}

	return &req, nil
}

// WriteResponse writes a JSON-RPC response.
func (t *StdioTransport) WriteResponse(resp *Response) error {
	t.writeMu.Lock()
	defer t.writeMu.Unlock()

	data, err := json.Marshal(resp)
	if err != nil {
		return fmt.Errorf("failed to marshal response: %w", err)
	}

	_, err = fmt.Fprintf(t.writer, "%s\n", data)
	return err
}

// MessageLoop runs the main message processing loop.
func MessageLoop(ctx context.Context, transport *StdioTransport, handler func(*Request) *Response) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		req, err := transport.ReadMessage()
		if err != nil {
			if err == io.EOF {
				return nil
			}
			errResp := &Response{
				JSONRPC: "2.0",
				Error: &ResponseError{
					Code:    ParseError,
					Message: "Parse error: " + err.Error(),
				},
			}
			transport.WriteResponse(errResp)
			continue
		}

		resp := handler(req)
		if resp != nil && req.ID != nil {
			resp.ID = req.ID
			resp.JSONRPC = "2.0"
			if err := transport.WriteResponse(resp); err != nil {
				return fmt.Errorf("failed to write response: %w", err)
			}
		}
	}
}

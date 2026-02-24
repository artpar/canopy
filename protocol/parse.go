package protocol

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
)

// Parser reads A2UI JSONL lines and emits typed messages.
type Parser struct {
	scanner *bufio.Scanner
}

func NewParser(r io.Reader) *Parser {
	s := bufio.NewScanner(r)
	s.Buffer(make([]byte, 0, 1024*1024), 10*1024*1024) // up to 10MB lines
	return &Parser{scanner: s}
}

// Message is a parsed A2UI message.
type Message struct {
	Type      MessageType
	SurfaceID string
	Body      interface{} // one of CreateSurface, DeleteSurface, UpdateComponents, UpdateDataModel, SetTheme
}

// Next reads the next JSONL line and returns a parsed Message.
// Returns io.EOF when done.
func (p *Parser) Next() (*Message, error) {
	if !p.scanner.Scan() {
		if err := p.scanner.Err(); err != nil {
			return nil, err
		}
		return nil, io.EOF
	}

	line := p.scanner.Bytes()
	if len(line) == 0 {
		return p.Next() // skip blank lines
	}

	var env Envelope
	if err := json.Unmarshal(line, &env); err != nil {
		return nil, fmt.Errorf("parse envelope: %w", err)
	}

	msg := &Message{
		Type:      env.Type,
		SurfaceID: env.SurfaceID,
	}

	switch env.Type {
	case MsgCreateSurface:
		var cs CreateSurface
		if err := json.Unmarshal(env.Payload, &cs); err != nil {
			return nil, fmt.Errorf("parse createSurface: %w", err)
		}
		msg.Body = cs

	case MsgDeleteSurface:
		var ds DeleteSurface
		if err := json.Unmarshal(env.Payload, &ds); err != nil {
			return nil, fmt.Errorf("parse deleteSurface: %w", err)
		}
		msg.Body = ds

	case MsgUpdateComponents:
		var uc UpdateComponents
		if err := json.Unmarshal(env.Payload, &uc); err != nil {
			return nil, fmt.Errorf("parse updateComponents: %w", err)
		}
		msg.Body = uc

	case MsgUpdateDataModel:
		var udm UpdateDataModel
		if err := json.Unmarshal(env.Payload, &udm); err != nil {
			return nil, fmt.Errorf("parse updateDataModel: %w", err)
		}
		msg.Body = udm

	case MsgSetTheme:
		var st SetTheme
		if err := json.Unmarshal(env.Payload, &st); err != nil {
			return nil, fmt.Errorf("parse setTheme: %w", err)
		}
		msg.Body = st

	case MsgTest:
		var tm TestMessage
		if err := json.Unmarshal(env.Payload, &tm); err != nil {
			return nil, fmt.Errorf("parse test: %w", err)
		}
		msg.Body = tm

	case MsgLoadLibrary:
		var ll LoadLibrary
		if err := json.Unmarshal(env.Payload, &ll); err != nil {
			return nil, fmt.Errorf("parse loadLibrary: %w", err)
		}
		msg.Body = ll

	case MsgLoadAssets:
		var la LoadAssets
		if err := json.Unmarshal(env.Payload, &la); err != nil {
			return nil, fmt.Errorf("parse loadAssets: %w", err)
		}
		msg.Body = la

	default:
		return nil, fmt.Errorf("unknown message type: %s", env.Type)
	}

	return msg, nil
}

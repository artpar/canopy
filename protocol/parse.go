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

	return ParseLine(line)
}

// ParseLine parses a single JSONL line into a Message.
func ParseLine(line []byte) (*Message, error) {
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

	case MsgDefineFunction:
		var df DefineFunction
		if err := json.Unmarshal(env.Payload, &df); err != nil {
			return nil, fmt.Errorf("parse defineFunction: %w", err)
		}
		msg.Body = df

	case MsgDefineComponent:
		var dc DefineComponent
		if err := json.Unmarshal(env.Payload, &dc); err != nil {
			return nil, fmt.Errorf("parse defineComponent: %w", err)
		}
		msg.Body = dc

	case MsgInclude:
		var inc Include
		if err := json.Unmarshal(env.Payload, &inc); err != nil {
			return nil, fmt.Errorf("parse include: %w", err)
		}
		msg.Body = inc

	case MsgCreateProcess:
		var cp CreateProcess
		if err := json.Unmarshal(env.Payload, &cp); err != nil {
			return nil, fmt.Errorf("parse createProcess: %w", err)
		}
		msg.Body = cp

	case MsgStopProcess:
		var sp StopProcess
		if err := json.Unmarshal(env.Payload, &sp); err != nil {
			return nil, fmt.Errorf("parse stopProcess: %w", err)
		}
		msg.Body = sp

	case MsgSendToProcess:
		var stp SendToProcess
		if err := json.Unmarshal(env.Payload, &stp); err != nil {
			return nil, fmt.Errorf("parse sendToProcess: %w", err)
		}
		msg.Body = stp

	case MsgCreateChannel:
		var cc CreateChannel
		if err := json.Unmarshal(env.Payload, &cc); err != nil {
			return nil, fmt.Errorf("parse createChannel: %w", err)
		}
		msg.Body = cc

	case MsgDeleteChannel:
		var dc DeleteChannel
		if err := json.Unmarshal(env.Payload, &dc); err != nil {
			return nil, fmt.Errorf("parse deleteChannel: %w", err)
		}
		msg.Body = dc

	case MsgPublish:
		var pub Publish
		if err := json.Unmarshal(env.Payload, &pub); err != nil {
			return nil, fmt.Errorf("parse publish: %w", err)
		}
		msg.Body = pub

	case MsgSubscribe:
		var sub Subscribe
		if err := json.Unmarshal(env.Payload, &sub); err != nil {
			return nil, fmt.Errorf("parse subscribe: %w", err)
		}
		msg.Body = sub

	case MsgUnsubscribe:
		var unsub Unsubscribe
		if err := json.Unmarshal(env.Payload, &unsub); err != nil {
			return nil, fmt.Errorf("parse unsubscribe: %w", err)
		}
		msg.Body = unsub

	default:
		return nil, fmt.Errorf("unknown message type: %s", env.Type)
	}

	return msg, nil
}

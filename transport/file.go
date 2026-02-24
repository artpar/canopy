package transport

import (
	"io"
	"jview/protocol"
	"log"
	"os"
	"sync"
)

// FileTransport reads A2UI JSONL from a file.
type FileTransport struct {
	path     string
	messages chan *protocol.Message
	errors   chan error
	done     chan struct{}
	stopOnce sync.Once
}

func NewFileTransport(path string) *FileTransport {
	return &FileTransport{
		path:     path,
		messages: make(chan *protocol.Message, 64),
		errors:   make(chan error, 8),
		done:     make(chan struct{}),
	}
}

func (f *FileTransport) Messages() <-chan *protocol.Message {
	return f.messages
}

func (f *FileTransport) Errors() <-chan error {
	return f.errors
}

func (f *FileTransport) Start() {
	go f.read()
}

func (f *FileTransport) Stop() {
	f.stopOnce.Do(func() { close(f.done) })
}

func (f *FileTransport) SendAction(surfaceID string, action *protocol.Action, data map[string]interface{}) {}

func (f *FileTransport) read() {
	defer close(f.messages)
	defer close(f.errors)

	file, err := os.Open(f.path)
	if err != nil {
		f.errors <- err
		return
	}
	defer file.Close()

	parser := protocol.NewParser(file)
	for {
		select {
		case <-f.done:
			return
		default:
		}

		msg, err := parser.Next()
		if err == io.EOF {
			log.Println("transport: file complete")
			return
		}
		if err != nil {
			f.errors <- err
			return
		}

		select {
		case f.messages <- msg:
		case <-f.done:
			return
		}
	}
}

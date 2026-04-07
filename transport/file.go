package transport

import (
	"fmt"
	"io"
	"canopy/jlog"
	"canopy/protocol"
	"os"
	"path/filepath"
	"sync"
)

// FileTransport reads A2UI JSONL from a file, with include support.
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

func (f *FileTransport) SendAction(surfaceID string, event *protocol.EventDef, data map[string]interface{}) {}

func (f *FileTransport) read() {
	defer close(f.messages)
	defer close(f.errors)

	absPath, err := filepath.Abs(f.path)
	if err != nil {
		f.errors <- err
		return
	}

	if err := f.readFile(absPath, nil); err != nil && err != io.EOF {
		f.errors <- err
	}
	jlog.Info("transport", "", "file complete")
}

// readFile reads a JSONL file, handling include messages recursively.
// includeStack tracks absolute paths for circular detection.
func (f *FileTransport) readFile(absPath string, includeStack []string) error {
	// Circular detection
	for _, p := range includeStack {
		if p == absPath {
			return fmt.Errorf("circular include detected: %s", absPath)
		}
	}

	// Max depth check
	if len(includeStack) >= 10 {
		return fmt.Errorf("include depth exceeded (max 10): %s", absPath)
	}

	file, err := os.Open(absPath)
	if err != nil {
		return err
	}
	defer file.Close()

	stack := append(includeStack, absPath)
	dir := filepath.Dir(absPath)
	parser := protocol.NewParser(file)

	for {
		select {
		case <-f.done:
			return nil
		default:
		}

		msg, err := parser.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			jlog.Warnf("transport", "", "skipping bad line in %s: %v", absPath, err)
			continue
		}

		// Handle include messages at the transport level
		if msg.Type == protocol.MsgInclude {
			inc := msg.Body.(protocol.Include)
			includePath := inc.Path
			if !filepath.IsAbs(includePath) {
				includePath = filepath.Join(dir, includePath)
			}
			absInclude, err := filepath.Abs(includePath)
			if err != nil {
				return fmt.Errorf("include path error: %w", err)
			}
			if err := f.readFile(absInclude, stack); err != nil {
				return fmt.Errorf("include %s: %w", inc.Path, err)
			}
			continue
		}

		select {
		case f.messages <- msg:
		case <-f.done:
			return nil
		}
	}
}

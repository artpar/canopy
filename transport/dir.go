package transport

import (
	"io"
	"canopy/jlog"
	"canopy/protocol"
	"os"
	"path/filepath"
	"sync"
)

// DirTransport reads multiple JSONL files from a directory in sorted order.
type DirTransport struct {
	dir      string
	files    []string // sorted basenames
	messages chan *protocol.Message
	errors   chan error
	done     chan struct{}
	stopOnce sync.Once
}

func NewDirTransport(dir string, files []string) *DirTransport {
	return &DirTransport{
		dir:      dir,
		files:    files,
		messages: make(chan *protocol.Message, 64),
		errors:   make(chan error, 8),
		done:     make(chan struct{}),
	}
}

func (d *DirTransport) Messages() <-chan *protocol.Message {
	return d.messages
}

func (d *DirTransport) Errors() <-chan error {
	return d.errors
}

func (d *DirTransport) Start() {
	go d.read()
}

func (d *DirTransport) Stop() {
	d.stopOnce.Do(func() { close(d.done) })
}

func (d *DirTransport) SendAction(surfaceID string, event *protocol.EventDef, data map[string]interface{}) {
}

func (d *DirTransport) read() {
	defer close(d.messages)
	defer close(d.errors)

	for _, name := range d.files {
		path := filepath.Join(d.dir, name)
		if err := d.readFile(path); err != nil {
			d.errors <- err
			return
		}
	}
	jlog.Info("transport", "", "directory complete")
}

func (d *DirTransport) readFile(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	// Use a FileTransport-like include handler for each file
	ft := &FileTransport{
		messages: d.messages,
		errors:   d.errors,
		done:     d.done,
	}

	absPath, err := filepath.Abs(path)
	if err != nil {
		return err
	}

	dir := filepath.Dir(absPath)
	parser := protocol.NewParser(file)

	for {
		select {
		case <-d.done:
			return nil
		default:
		}

		msg, err := parser.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			jlog.Warnf("transport", "", "skipping bad line in %s: %v", path, err)
			continue
		}

		// Handle include messages
		if msg.Type == protocol.MsgInclude {
			inc := msg.Body.(protocol.Include)
			includePath := inc.Path
			if !filepath.IsAbs(includePath) {
				includePath = filepath.Join(dir, includePath)
			}
			absInclude, err := filepath.Abs(includePath)
			if err != nil {
				return err
			}
			if err := ft.readFile(absInclude, []string{absPath}); err != nil {
				return err
			}
			continue
		}

		select {
		case d.messages <- msg:
		case <-d.done:
			return nil
		}
	}
}

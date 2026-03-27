package transport

import (
	"io"
	"jview/jlog"
	"jview/protocol"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// WatchTransport reads JSONL files and re-reads them when they change.
// On reload, it sends deleteSurface for all surfaces created in the previous
// read, then re-reads the files from scratch.
type WatchTransport struct {
	path     string // file or directory
	messages chan *protocol.Message
	errors   chan error
	done     chan struct{}
	stopOnce sync.Once

	mu       sync.Mutex
	surfaces []string // surfaceIDs created during current read
}

func NewWatchTransport(path string) *WatchTransport {
	return &WatchTransport{
		path:     path,
		messages: make(chan *protocol.Message, 64),
		errors:   make(chan error, 8),
		done:     make(chan struct{}),
	}
}

func (w *WatchTransport) Messages() <-chan *protocol.Message {
	return w.messages
}

func (w *WatchTransport) Errors() <-chan error {
	return w.errors
}

func (w *WatchTransport) Start() {
	go w.run()
}

func (w *WatchTransport) Stop() {
	w.stopOnce.Do(func() { close(w.done) })
}

func (w *WatchTransport) SendAction(surfaceID string, event *protocol.EventDef, data map[string]interface{}) {
}

func (w *WatchTransport) run() {
	defer close(w.messages)
	defer close(w.errors)

	// Initial read
	if err := w.readAll(); err != nil {
		w.errors <- err
		return
	}

	// Collect initial mod times
	lastMods := w.collectModTimes()

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-w.done:
			return
		case <-ticker.C:
			mods := w.collectModTimes()
			if !modsEqual(lastMods, mods) {
				jlog.Info("transport", "", "watch: file change detected, reloading")
				lastMods = mods
				w.reload()
			}
		}
	}
}

// reload tears down existing surfaces and re-reads all files.
func (w *WatchTransport) reload() {
	w.mu.Lock()
	surfaces := w.surfaces
	w.surfaces = nil
	w.mu.Unlock()

	// Delete existing surfaces in reverse order
	for i := len(surfaces) - 1; i >= 0; i-- {
		msg := &protocol.Message{
			Type:      protocol.MsgDeleteSurface,
			SurfaceID: surfaces[i],
			Body:      protocol.DeleteSurface{SurfaceID: surfaces[i]},
		}
		select {
		case w.messages <- msg:
		case <-w.done:
			return
		}
	}

	if err := w.readAll(); err != nil {
		jlog.Errorf("transport", "", "watch: reload error: %v", err)
	}
}

// readAll reads the file or directory, tracking createSurface messages.
func (w *WatchTransport) readAll() error {
	files, err := w.resolveFiles()
	if err != nil {
		return err
	}

	for _, path := range files {
		if err := w.readFile(path); err != nil {
			return err
		}
	}
	return nil
}

func (w *WatchTransport) readFile(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	dir := filepath.Dir(path)
	parser := protocol.NewParser(file)

	for {
		select {
		case <-w.done:
			return nil
		default:
		}

		msg, err := parser.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			jlog.Warnf("transport", "", "watch: skipping bad line in %s: %v", path, err)
			continue
		}

		// Handle includes
		if msg.Type == protocol.MsgInclude {
			inc := msg.Body.(protocol.Include)
			includePath := inc.Path
			if !filepath.IsAbs(includePath) {
				includePath = filepath.Join(dir, includePath)
			}
			if err := w.readFile(includePath); err != nil {
				return err
			}
			continue
		}

		// Track surface creation for teardown on reload
		if msg.Type == protocol.MsgCreateSurface {
			cs := msg.Body.(protocol.CreateSurface)
			w.mu.Lock()
			w.surfaces = append(w.surfaces, cs.SurfaceID)
			w.mu.Unlock()
		}

		select {
		case w.messages <- msg:
		case <-w.done:
			return nil
		}
	}
}

// resolveFiles returns the ordered list of JSONL files to read.
func (w *WatchTransport) resolveFiles() ([]string, error) {
	info, err := os.Stat(w.path)
	if err != nil {
		return nil, err
	}

	if !info.IsDir() {
		abs, err := filepath.Abs(w.path)
		if err != nil {
			return nil, err
		}
		return []string{abs}, nil
	}

	// Directory: prefer app.jsonl or main.jsonl
	for _, entry := range []string{"app.jsonl", "main.jsonl"} {
		ep := filepath.Join(w.path, entry)
		if _, err := os.Stat(ep); err == nil {
			abs, err := filepath.Abs(ep)
			if err != nil {
				return nil, err
			}
			return []string{abs}, nil
		}
	}

	// Fallback: all .jsonl files sorted
	entries, err := os.ReadDir(w.path)
	if err != nil {
		return nil, err
	}
	var files []string
	for _, e := range entries {
		if !e.IsDir() && filepath.Ext(e.Name()) == ".jsonl" {
			abs, err := filepath.Abs(filepath.Join(w.path, e.Name()))
			if err != nil {
				return nil, err
			}
			files = append(files, abs)
		}
	}
	sort.Strings(files)
	return files, nil
}

// collectModTimes gathers mod times for all JSONL files being watched.
func (w *WatchTransport) collectModTimes() map[string]time.Time {
	mods := make(map[string]time.Time)
	files, err := w.resolveFiles()
	if err != nil {
		return mods
	}
	for _, f := range files {
		if info, err := os.Stat(f); err == nil {
			mods[f] = info.ModTime()
		}
	}

	// Also check included files by scanning for includes
	// (simplified: just check all .jsonl in the directory)
	info, _ := os.Stat(w.path)
	if info != nil && info.IsDir() {
		entries, _ := os.ReadDir(w.path)
		for _, e := range entries {
			if !e.IsDir() && filepath.Ext(e.Name()) == ".jsonl" {
				p := filepath.Join(w.path, e.Name())
				if info, err := os.Stat(p); err == nil {
					mods[p] = info.ModTime()
				}
			}
		}
	} else if info != nil {
		// Single file: also check sibling .jsonl files (includes)
		dir := filepath.Dir(w.path)
		entries, _ := os.ReadDir(dir)
		for _, e := range entries {
			if !e.IsDir() && filepath.Ext(e.Name()) == ".jsonl" {
				p := filepath.Join(dir, e.Name())
				if info, err := os.Stat(p); err == nil {
					mods[p] = info.ModTime()
				}
			}
		}
	}

	return mods
}

func modsEqual(a, b map[string]time.Time) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if bv, ok := b[k]; !ok || !v.Equal(bv) {
			return false
		}
	}
	return true
}

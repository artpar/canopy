package transport

import (
	"jview/protocol"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func fixtureDir() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "testdata")
}

func TestFileTransportHappyPath(t *testing.T) {
	defer goroutineLeakCheck(t)()

	ft := NewFileTransport(filepath.Join(fixtureDir(), "hello.jsonl"))
	ft.Start()

	msgs := drainWithTimeout(t, ft.Messages(), 5*time.Second)
	if len(msgs) == 0 {
		t.Fatal("expected at least one message")
	}

	// Errors channel must also close
	errs := drainErrorsWithTimeout(t, ft.Errors(), time.Second)
	if len(errs) != 0 {
		t.Errorf("unexpected errors: %v", errs)
	}
}

func TestFileTransportMissingFile(t *testing.T) {
	defer goroutineLeakCheck(t)()

	ft := NewFileTransport("/nonexistent/path/to/file.jsonl")
	ft.Start()

	// Messages channel must close (no messages)
	msgs := drainWithTimeout(t, ft.Messages(), 5*time.Second)
	if len(msgs) != 0 {
		t.Errorf("expected 0 messages from missing file, got %d", len(msgs))
	}

	// Errors channel must close with one error
	errs := drainErrorsWithTimeout(t, ft.Errors(), time.Second)
	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d", len(errs))
	}
}

func TestFileTransportStopBeforeComplete(t *testing.T) {
	defer goroutineLeakCheck(t)()

	ft := NewFileTransport(filepath.Join(fixtureDir(), "hello.jsonl"))
	ft.Start()

	// Stop immediately before reading all messages
	ft.Stop()

	// Both channels must close
	timer := time.NewTimer(5 * time.Second)
	defer timer.Stop()
	for {
		select {
		case _, ok := <-ft.Messages():
			if !ok {
				goto msgsDone
			}
		case <-timer.C:
			t.Fatal("messages channel did not close after Stop()")
		}
	}
msgsDone:
	for {
		select {
		case _, ok := <-ft.Errors():
			if !ok {
				return
			}
		case <-timer.C:
			t.Fatal("errors channel did not close after Stop()")
		}
	}
}

func TestFileTransportMalformedJSON(t *testing.T) {
	defer goroutineLeakCheck(t)()

	// Create a temp file with bad JSON — transport now skips bad lines
	tmp, err := os.CreateTemp("", "jview-test-*.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmp.Name())
	tmp.WriteString("this is not json\n")
	tmp.Close()

	ft := NewFileTransport(tmp.Name())
	ft.Start()

	// Messages closes with 0 messages since only line is bad
	msgs := drainWithTimeout(t, ft.Messages(), 5*time.Second)
	if len(msgs) != 0 {
		t.Errorf("expected 0 messages from all-bad file, got %d", len(msgs))
	}

	// Errors channel must close with no errors (bad lines are logged and skipped)
	errs := drainErrorsWithTimeout(t, ft.Errors(), time.Second)
	if len(errs) != 0 {
		t.Errorf("expected 0 errors (bad lines are skipped), got %d: %v", len(errs), errs)
	}
}

func TestFileTransportMalformedMidFile(t *testing.T) {
	defer goroutineLeakCheck(t)()

	// Create a file with valid JSONL, a bad line, then more valid JSONL
	tmp, err := os.CreateTemp("", "jview-test-*.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmp.Name())
	tmp.WriteString(`{"type":"createSurface","surfaceId":"s1","title":"First","width":400,"height":300}` + "\n")
	tmp.WriteString("this is garbage\n")
	tmp.WriteString(`{"type":"createSurface","surfaceId":"s2","title":"Second","width":400,"height":300}` + "\n")
	tmp.Close()

	ft := NewFileTransport(tmp.Name())
	ft.Start()

	msgs := drainWithTimeout(t, ft.Messages(), 5*time.Second)
	if len(msgs) != 2 {
		t.Fatalf("expected 2 valid messages (bad line skipped), got %d", len(msgs))
	}
	if msgs[0].Type != protocol.MsgCreateSurface {
		t.Errorf("msg[0].Type = %q, want createSurface", msgs[0].Type)
	}
	if msgs[1].Type != protocol.MsgCreateSurface {
		t.Errorf("msg[1].Type = %q, want createSurface", msgs[1].Type)
	}

	errs := drainErrorsWithTimeout(t, ft.Errors(), time.Second)
	if len(errs) != 0 {
		t.Errorf("expected 0 errors, got %d: %v", len(errs), errs)
	}
}

func TestFileTransportEmptyFile(t *testing.T) {
	defer goroutineLeakCheck(t)()

	tmp, err := os.CreateTemp("", "jview-test-*.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmp.Name())
	tmp.Close()

	ft := NewFileTransport(tmp.Name())
	ft.Start()

	msgs := drainWithTimeout(t, ft.Messages(), 5*time.Second)
	if len(msgs) != 0 {
		t.Errorf("expected 0 messages from empty file, got %d", len(msgs))
	}

	errs := drainErrorsWithTimeout(t, ft.Errors(), time.Second)
	if len(errs) != 0 {
		t.Errorf("unexpected errors: %v", errs)
	}
}

func TestFileTransportInclude(t *testing.T) {
	defer goroutineLeakCheck(t)()

	ft := NewFileTransport(filepath.Join(fixtureDir(), "includes", "main.jsonl"))
	ft.Start()

	msgs := drainWithTimeout(t, ft.Messages(), 5*time.Second)
	errs := drainErrorsWithTimeout(t, ft.Errors(), time.Second)

	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}

	// main.jsonl includes defs.jsonl (1 defineFunction message), then has createSurface + updateComponents
	// So we should get: defineFunction, createSurface, updateComponents = 3 messages
	if len(msgs) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(msgs))
	}
	if msgs[0].Type != protocol.MsgDefineFunction {
		t.Errorf("msg[0].Type = %q, want defineFunction", msgs[0].Type)
	}
	if msgs[1].Type != protocol.MsgCreateSurface {
		t.Errorf("msg[1].Type = %q, want createSurface", msgs[1].Type)
	}
	if msgs[2].Type != protocol.MsgUpdateComponents {
		t.Errorf("msg[2].Type = %q, want updateComponents", msgs[2].Type)
	}
}

func TestFileTransportCircularInclude(t *testing.T) {
	defer goroutineLeakCheck(t)()

	// Create two files that include each other
	dir := t.TempDir()
	a := filepath.Join(dir, "a.jsonl")
	b := filepath.Join(dir, "b.jsonl")
	os.WriteFile(a, []byte(`{"type":"include","path":"b.jsonl"}`+"\n"), 0644)
	os.WriteFile(b, []byte(`{"type":"include","path":"a.jsonl"}`+"\n"), 0644)

	ft := NewFileTransport(a)
	ft.Start()

	drainWithTimeout(t, ft.Messages(), 5*time.Second)
	errs := drainErrorsWithTimeout(t, ft.Errors(), time.Second)

	if len(errs) == 0 {
		t.Fatal("expected circular include error")
	}
}

func TestFileTransportIncludeDepthLimit(t *testing.T) {
	defer goroutineLeakCheck(t)()

	// Create a chain of 12 includes (exceeds limit of 10)
	dir := t.TempDir()
	for i := 0; i < 12; i++ {
		name := filepath.Join(dir, filepath.Base(filepath.Join(dir, "file"+string(rune('a'+i))+".jsonl")))
		var content string
		if i < 11 {
			next := filepath.Join(dir, "file"+string(rune('a'+i+1))+".jsonl")
			content = `{"type":"include","path":"` + filepath.Base(next) + `"}` + "\n"
		} else {
			content = `{"type":"createSurface","surfaceId":"s","title":"deep"}` + "\n"
		}
		os.WriteFile(name, []byte(content), 0644)
	}

	ft := NewFileTransport(filepath.Join(dir, "filea.jsonl"))
	ft.Start()

	drainWithTimeout(t, ft.Messages(), 5*time.Second)
	errs := drainErrorsWithTimeout(t, ft.Errors(), time.Second)

	if len(errs) == 0 {
		t.Fatal("expected depth limit error")
	}
}

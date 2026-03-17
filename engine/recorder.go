package engine

import (
	"jview/jlog"
	"jview/protocol"
	"os"
)

// Recorder writes raw JSONL lines to a file for cache/replay purposes.
// It only records UI-definition message types (not runtime messages like
// process/channel, test, or include).
type Recorder struct {
	f *os.File
}

// NewRecorder creates a recorder that writes to the given file.
func NewRecorder(f *os.File) *Recorder {
	return &Recorder{f: f}
}

// Record writes the message's raw JSONL line if it is a recordable type.
// No-op if the recorder or message is nil.
func (r *Recorder) Record(msg *protocol.Message) {
	if r == nil || msg == nil || len(msg.RawLine) == 0 {
		return
	}
	if !isRecordable(msg.Type) {
		return
	}
	if _, err := r.f.Write(msg.RawLine); err != nil {
		jlog.Errorf("recorder", "", "write error: %v", err)
		return
	}
	r.f.Write([]byte("\n"))
}

// Close closes the underlying file.
func (r *Recorder) Close() error {
	if r == nil || r.f == nil {
		return nil
	}
	return r.f.Close()
}

// isRecordable returns true for message types that define the UI and should
// be recorded for caching. Skips runtime-only messages (test, include,
// process, channel).
func isRecordable(t protocol.MessageType) bool {
	switch t {
	case protocol.MsgCreateSurface,
		protocol.MsgUpdateComponents,
		protocol.MsgUpdateDataModel,
		protocol.MsgDefineFunction,
		protocol.MsgDefineComponent,
		protocol.MsgSetTheme,
		protocol.MsgUpdateMenu,
		protocol.MsgUpdateToolbar,
		protocol.MsgUpdateWindow,
		protocol.MsgLoadAssets,
		protocol.MsgLoadLibrary:
		return true
	default:
		return false
	}
}

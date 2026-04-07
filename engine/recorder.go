package engine

import (
	"canopy/jlog"
	"canopy/protocol"
	"os"
)

// Recorder writes raw JSONL lines to a file for cache/replay purposes.
// It only records UI-definition message types (not runtime messages like
// process/channel, test, or include).
//
// When the LLM sends a duplicate createSurface for a surface that was already
// recorded, the recorder truncates back to the start and re-records from the
// new createSurface. This prevents stale partial messages from corrupting the
// cache on replay.
type Recorder struct {
	f        *os.File
	surfaces map[string]bool // tracks which surfaceIds have been created
}

// NewRecorder creates a recorder that writes to the given file.
func NewRecorder(f *os.File) *Recorder {
	return &Recorder{f: f, surfaces: make(map[string]bool)}
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

	// If this is a duplicate createSurface, the LLM is re-doing the layout.
	// Truncate the file and start fresh so the cache only contains the final version.
	if msg.Type == protocol.MsgCreateSurface {
		cs := msg.Body.(protocol.CreateSurface)
		if r.surfaces[cs.SurfaceID] {
			jlog.Infof("recorder", cs.SurfaceID, "duplicate createSurface — truncating cache to re-record")
			r.f.Truncate(0)
			r.f.Seek(0, 0)
			r.surfaces = make(map[string]bool)
		}
		r.surfaces[cs.SurfaceID] = true
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

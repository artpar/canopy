package mcp

import (
	"encoding/json"
)

func (s *Server) registerSystemTools() {
	s.registerNotify()
	s.registerClipboardRead()
	s.registerClipboardWrite()
	s.registerOpenURL()
	s.registerFileOpen()
	s.registerFileSave()
	s.registerFileRead()
	s.registerFileWrite()
	s.registerFileAppend()
	s.registerAlert()
	s.registerCameraCaptureSystem()
	s.registerAudioRecordStartSystem()
	s.registerAudioRecordStopSystem()
	s.registerScreenCaptureSystem()
	s.registerScreenRecordStartSystem()
	s.registerScreenRecordStopSystem()
}

func (s *Server) registerNotify() {
	s.register("notify", "Send a macOS notification", json.RawMessage(`{
		"type": "object",
		"properties": {
			"title":    {"type": "string", "description": "Notification title"},
			"body":     {"type": "string", "description": "Notification body text"},
			"subtitle": {"type": "string", "description": "Optional subtitle"}
		},
		"required": ["title", "body"],
		"additionalProperties": false
	}`), func(args json.RawMessage) *ToolCallResult {
		np := s.sess.NativeProvider()
		if np == nil {
			return errorResult("native provider not available")
		}
		var p struct {
			Title    string `json:"title"`
			Body     string `json:"body"`
			Subtitle string `json:"subtitle"`
		}
		if err := json.Unmarshal(args, &p); err != nil {
			return errorResult("invalid args: " + err.Error())
		}
		if err := np.Notify(p.Title, p.Body, p.Subtitle); err != nil {
			return errorResult("notify: " + err.Error())
		}
		return textResult("notification sent")
	})
}

func (s *Server) registerClipboardRead() {
	s.register("clipboard_read", "Read text from the system clipboard", json.RawMessage(`{
		"type": "object",
		"properties": {},
		"additionalProperties": false
	}`), func(args json.RawMessage) *ToolCallResult {
		np := s.sess.NativeProvider()
		if np == nil {
			return errorResult("native provider not available")
		}
		text, err := np.ClipboardRead()
		if err != nil {
			return errorResult("clipboard_read: " + err.Error())
		}
		return textResult(text)
	})
}

func (s *Server) registerClipboardWrite() {
	s.register("clipboard_write", "Write text to the system clipboard", json.RawMessage(`{
		"type": "object",
		"properties": {
			"text": {"type": "string", "description": "Text to write to clipboard"}
		},
		"required": ["text"],
		"additionalProperties": false
	}`), func(args json.RawMessage) *ToolCallResult {
		np := s.sess.NativeProvider()
		if np == nil {
			return errorResult("native provider not available")
		}
		var p struct {
			Text string `json:"text"`
		}
		if err := json.Unmarshal(args, &p); err != nil {
			return errorResult("invalid args: " + err.Error())
		}
		if err := np.ClipboardWrite(p.Text); err != nil {
			return errorResult("clipboard_write: " + err.Error())
		}
		return textResult("copied")
	})
}

func (s *Server) registerOpenURL() {
	s.register("open_url", "Open a URL or file path in the default application", json.RawMessage(`{
		"type": "object",
		"properties": {
			"url": {"type": "string", "description": "URL or file path to open"}
		},
		"required": ["url"],
		"additionalProperties": false
	}`), func(args json.RawMessage) *ToolCallResult {
		np := s.sess.NativeProvider()
		if np == nil {
			return errorResult("native provider not available")
		}
		var p struct {
			URL string `json:"url"`
		}
		if err := json.Unmarshal(args, &p); err != nil {
			return errorResult("invalid args: " + err.Error())
		}
		if err := np.OpenURL(p.URL); err != nil {
			return errorResult("open_url: " + err.Error())
		}
		return textResult("opened")
	})
}

func (s *Server) registerFileOpen() {
	s.register("file_open", "Show a file open dialog and return selected path(s)", json.RawMessage(`{
		"type": "object",
		"properties": {
			"title":          {"type": "string", "description": "Dialog title"},
			"allowed_types":  {"type": "string", "description": "Comma-separated file extensions (e.g. 'txt,md,json')"},
			"allow_multiple": {"type": "boolean", "description": "Allow selecting multiple files"}
		},
		"additionalProperties": false
	}`), func(args json.RawMessage) *ToolCallResult {
		np := s.sess.NativeProvider()
		if np == nil {
			return errorResult("native provider not available")
		}
		var p struct {
			Title        string `json:"title"`
			AllowedTypes string `json:"allowed_types"`
			AllowMulti   bool   `json:"allow_multiple"`
		}
		if err := json.Unmarshal(args, &p); err != nil {
			return errorResult("invalid args: " + err.Error())
		}
		if p.Title == "" {
			p.Title = "Open"
		}
		paths, err := np.FileOpen(p.Title, p.AllowedTypes, p.AllowMulti)
		if err != nil {
			return errorResult("file_open: " + err.Error())
		}
		if paths == nil {
			return textResult("cancelled")
		}
		cb, err := JSONContent(paths)
		if err != nil {
			return errorResult(err.Error())
		}
		return &ToolCallResult{Content: []ContentBlock{cb}}
	})
}

func (s *Server) registerFileSave() {
	s.register("file_save", "Show a file save dialog and return selected path", json.RawMessage(`{
		"type": "object",
		"properties": {
			"title":         {"type": "string", "description": "Dialog title"},
			"default_name":  {"type": "string", "description": "Default file name"},
			"allowed_types": {"type": "string", "description": "Comma-separated file extensions"}
		},
		"additionalProperties": false
	}`), func(args json.RawMessage) *ToolCallResult {
		np := s.sess.NativeProvider()
		if np == nil {
			return errorResult("native provider not available")
		}
		var p struct {
			Title        string `json:"title"`
			DefaultName  string `json:"default_name"`
			AllowedTypes string `json:"allowed_types"`
		}
		if err := json.Unmarshal(args, &p); err != nil {
			return errorResult("invalid args: " + err.Error())
		}
		if p.Title == "" {
			p.Title = "Save"
		}
		path, err := np.FileSave(p.Title, p.DefaultName, p.AllowedTypes)
		if err != nil {
			return errorResult("file_save: " + err.Error())
		}
		if path == "" {
			return textResult("cancelled")
		}
		return textResult(path)
	})
}

func (s *Server) registerFileRead() {
	s.register("file_read", "Read file contents as string", json.RawMessage(`{
		"type": "object",
		"properties": {
			"path": {"type": "string", "description": "File path to read"}
		},
		"required": ["path"],
		"additionalProperties": false
	}`), func(args json.RawMessage) *ToolCallResult {
		np := s.sess.NativeProvider()
		if np == nil {
			return errorResult("native provider not available")
		}
		var p struct {
			Path string `json:"path"`
		}
		if err := json.Unmarshal(args, &p); err != nil {
			return errorResult("invalid args: " + err.Error())
		}
		content, err := np.FileRead(p.Path)
		if err != nil {
			return errorResult("file_read: " + err.Error())
		}
		return textResult(content)
	})
}

func (s *Server) registerFileWrite() {
	s.register("file_write", "Write content to file (creates or overwrites)", json.RawMessage(`{
		"type": "object",
		"properties": {
			"path":    {"type": "string", "description": "File path to write"},
			"content": {"type": "string", "description": "Content to write"}
		},
		"required": ["path", "content"],
		"additionalProperties": false
	}`), func(args json.RawMessage) *ToolCallResult {
		np := s.sess.NativeProvider()
		if np == nil {
			return errorResult("native provider not available")
		}
		var p struct {
			Path    string `json:"path"`
			Content string `json:"content"`
		}
		if err := json.Unmarshal(args, &p); err != nil {
			return errorResult("invalid args: " + err.Error())
		}
		if err := np.FileWrite(p.Path, p.Content); err != nil {
			return errorResult("file_write: " + err.Error())
		}
		return textResult("ok")
	})
}

func (s *Server) registerFileAppend() {
	s.register("file_append", "Append content to file (creates if missing)", json.RawMessage(`{
		"type": "object",
		"properties": {
			"path":    {"type": "string", "description": "File path to append to"},
			"content": {"type": "string", "description": "Content to append"}
		},
		"required": ["path", "content"],
		"additionalProperties": false
	}`), func(args json.RawMessage) *ToolCallResult {
		np := s.sess.NativeProvider()
		if np == nil {
			return errorResult("native provider not available")
		}
		var p struct {
			Path    string `json:"path"`
			Content string `json:"content"`
		}
		if err := json.Unmarshal(args, &p); err != nil {
			return errorResult("invalid args: " + err.Error())
		}
		if err := np.FileAppend(p.Path, p.Content); err != nil {
			return errorResult("file_append: " + err.Error())
		}
		return textResult("ok")
	})
}

func (s *Server) registerAlert() {
	s.register("alert", "Show a modal alert dialog", json.RawMessage(`{
		"type": "object",
		"properties": {
			"title":   {"type": "string", "description": "Alert title"},
			"message": {"type": "string", "description": "Alert message"},
			"style":   {"type": "string", "enum": ["informational", "warning", "critical"], "description": "Alert style"},
			"buttons": {"type": "array", "items": {"type": "string"}, "description": "Button titles (first is default)"}
		},
		"required": ["title", "message"],
		"additionalProperties": false
	}`), func(args json.RawMessage) *ToolCallResult {
		np := s.sess.NativeProvider()
		if np == nil {
			return errorResult("native provider not available")
		}
		var p struct {
			Title   string   `json:"title"`
			Message string   `json:"message"`
			Style   string   `json:"style"`
			Buttons []string `json:"buttons"`
		}
		if err := json.Unmarshal(args, &p); err != nil {
			return errorResult("invalid args: " + err.Error())
		}
		if p.Style == "" {
			p.Style = "informational"
		}
		idx, err := np.Alert(p.Title, p.Message, p.Style, p.Buttons)
		if err != nil {
			return errorResult("alert: " + err.Error())
		}
		cb, err := JSONContent(map[string]int{"button_index": idx})
		if err != nil {
			return errorResult(err.Error())
		}
		return &ToolCallResult{Content: []ContentBlock{cb}}
	})
}

func (s *Server) registerCameraCaptureSystem() {
	s.register("camera_capture_headless", "Take a photo using the camera (no preview)", json.RawMessage(`{
		"type": "object",
		"properties": {
			"device_position": {"type": "string", "enum": ["front", "back"], "description": "Camera position (default: front)"}
		},
		"additionalProperties": false
	}`), func(args json.RawMessage) *ToolCallResult {
		np := s.sess.NativeProvider()
		if np == nil {
			return errorResult("native provider not available")
		}
		var p struct {
			DevicePosition string `json:"device_position"`
		}
		if err := json.Unmarshal(args, &p); err != nil {
			return errorResult("invalid args: " + err.Error())
		}
		if p.DevicePosition == "" {
			p.DevicePosition = "front"
		}
		path, err := np.CameraCapture(p.DevicePosition)
		if err != nil {
			return errorResult("camera_capture: " + err.Error())
		}
		return textResult(path)
	})
}

func (s *Server) registerAudioRecordStartSystem() {
	s.register("audio_record_start", "Start recording audio from the microphone", json.RawMessage(`{
		"type": "object",
		"properties": {
			"format":      {"type": "string", "enum": ["m4a", "wav"], "description": "Audio format (default: m4a)"},
			"sample_rate": {"type": "number", "description": "Sample rate in Hz (default: 44100)"},
			"channels":    {"type": "integer", "description": "Number of channels (default: 1)"}
		},
		"additionalProperties": false
	}`), func(args json.RawMessage) *ToolCallResult {
		np := s.sess.NativeProvider()
		if np == nil {
			return errorResult("native provider not available")
		}
		var p struct {
			Format     string  `json:"format"`
			SampleRate float64 `json:"sample_rate"`
			Channels   int     `json:"channels"`
		}
		if err := json.Unmarshal(args, &p); err != nil {
			return errorResult("invalid args: " + err.Error())
		}
		if p.Format == "" {
			p.Format = "m4a"
		}
		if p.SampleRate == 0 {
			p.SampleRate = 44100
		}
		if p.Channels == 0 {
			p.Channels = 1
		}
		id, err := np.AudioRecordStart(p.Format, p.SampleRate, p.Channels)
		if err != nil {
			return errorResult("audio_record_start: " + err.Error())
		}
		return textResult(id)
	})
}

func (s *Server) registerAudioRecordStopSystem() {
	s.register("audio_record_stop", "Stop a recording and return the file path", json.RawMessage(`{
		"type": "object",
		"properties": {
			"recording_id": {"type": "string", "description": "Recording ID from audio_record_start"}
		},
		"required": ["recording_id"],
		"additionalProperties": false
	}`), func(args json.RawMessage) *ToolCallResult {
		np := s.sess.NativeProvider()
		if np == nil {
			return errorResult("native provider not available")
		}
		var p struct {
			RecordingID string `json:"recording_id"`
		}
		if err := json.Unmarshal(args, &p); err != nil {
			return errorResult("invalid args: " + err.Error())
		}
		path, err := np.AudioRecordStop(p.RecordingID)
		if err != nil {
			return errorResult("audio_record_stop: " + err.Error())
		}
		return textResult(path)
	})
}

func (s *Server) registerScreenCaptureSystem() {
	s.register("screen_capture", "Take a screenshot of the entire screen", json.RawMessage(`{
		"type": "object",
		"properties": {
			"capture_type": {"type": "string", "enum": ["screen"], "description": "Capture type (default: screen)"}
		},
		"additionalProperties": false
	}`), func(args json.RawMessage) *ToolCallResult {
		np := s.sess.NativeProvider()
		if np == nil {
			return errorResult("native provider not available")
		}
		var p struct {
			CaptureType string `json:"capture_type"`
		}
		if err := json.Unmarshal(args, &p); err != nil {
			return errorResult("invalid args: " + err.Error())
		}
		if p.CaptureType == "" {
			p.CaptureType = "screen"
		}
		path, err := np.ScreenCapture(p.CaptureType)
		if err != nil {
			return errorResult("screen_capture: " + err.Error())
		}
		return textResult(path)
	})
}

func (s *Server) registerScreenRecordStartSystem() {
	s.register("screen_record_start", "Start recording the screen", json.RawMessage(`{
		"type": "object",
		"properties": {
			"capture_type": {"type": "string", "enum": ["screen"], "description": "Capture type (default: screen)"}
		},
		"additionalProperties": false
	}`), func(args json.RawMessage) *ToolCallResult {
		np := s.sess.NativeProvider()
		if np == nil {
			return errorResult("native provider not available")
		}
		var p struct {
			CaptureType string `json:"capture_type"`
		}
		if err := json.Unmarshal(args, &p); err != nil {
			return errorResult("invalid args: " + err.Error())
		}
		if p.CaptureType == "" {
			p.CaptureType = "screen"
		}
		id, err := np.ScreenRecordStart(p.CaptureType)
		if err != nil {
			return errorResult("screen_record_start: " + err.Error())
		}
		return textResult(id)
	})
}

func (s *Server) registerScreenRecordStopSystem() {
	s.register("screen_record_stop", "Stop recording the screen and return the video file path", json.RawMessage(`{
		"type": "object",
		"properties": {
			"recording_id": {"type": "string", "description": "Recording ID from screen_record_start"}
		},
		"required": ["recording_id"],
		"additionalProperties": false
	}`), func(args json.RawMessage) *ToolCallResult {
		np := s.sess.NativeProvider()
		if np == nil {
			return errorResult("native provider not available")
		}
		var p struct {
			RecordingID string `json:"recording_id"`
		}
		if err := json.Unmarshal(args, &p); err != nil {
			return errorResult("invalid args: " + err.Error())
		}
		path, err := np.ScreenRecordStop(p.RecordingID)
		if err != nil {
			return errorResult("screen_record_stop: " + err.Error())
		}
		return textResult(path)
	})
}

func textResult(text string) *ToolCallResult {
	return &ToolCallResult{Content: []ContentBlock{{Type: "text", Text: text}}}
}

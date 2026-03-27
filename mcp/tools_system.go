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
	s.registerAlert()
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

func textResult(text string) *ToolCallResult {
	return &ToolCallResult{Content: []ContentBlock{{Type: "text", Text: text}}}
}

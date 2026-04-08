package engine

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

var httpClient = &http.Client{Timeout: 30 * time.Second}

func (e *Evaluator) fnNotify(args []any) (any, error) {
	if e.Native == nil {
		return nil, fmt.Errorf("notify: not available in headless mode")
	}
	if len(args) < 2 {
		return nil, fmt.Errorf("notify requires at least 2 args (title, body)")
	}
	title := toString(args[0])
	body := toString(args[1])
	subtitle := ""
	if len(args) >= 3 {
		subtitle = toString(args[2])
	}
	if err := e.Native.Notify(title, body, subtitle); err != nil {
		return "unavailable", nil
	}
	return "sent", nil
}

func (e *Evaluator) fnClipboardRead(args []any) (any, error) {
	if e.Native == nil {
		return nil, fmt.Errorf("clipboardRead: not available in headless mode")
	}
	return e.Native.ClipboardRead()
}

func (e *Evaluator) fnClipboardWrite(args []any) (any, error) {
	if e.Native == nil {
		return nil, fmt.Errorf("clipboardWrite: not available in headless mode")
	}
	if len(args) < 1 {
		return nil, fmt.Errorf("clipboardWrite requires 1 arg (text)")
	}
	if err := e.Native.ClipboardWrite(toString(args[0])); err != nil {
		return nil, err
	}
	return "copied", nil
}

func (e *Evaluator) fnOpenURL(args []any) (any, error) {
	if e.Native == nil {
		return nil, fmt.Errorf("openURL: not available in headless mode")
	}
	if len(args) < 1 {
		return nil, fmt.Errorf("openURL requires 1 arg (url)")
	}
	if err := e.Native.OpenURL(toString(args[0])); err != nil {
		return nil, err
	}
	return "opened", nil
}

func (e *Evaluator) fnFileOpen(args []any) (any, error) {
	if e.Native == nil {
		return nil, fmt.Errorf("fileOpen: not available in headless mode")
	}
	title := "Open"
	allowedTypes := ""
	allowMultiple := false
	if len(args) >= 1 {
		title = toString(args[0])
	}
	if len(args) >= 2 {
		allowedTypes = toString(args[1])
	}
	if len(args) >= 3 {
		allowMultiple, _ = toBool(args[2])
	}
	paths, err := e.Native.FileOpen(title, allowedTypes, allowMultiple)
	if err != nil {
		return nil, err
	}
	if paths == nil {
		return "", nil
	}
	if len(paths) == 1 {
		return paths[0], nil
	}
	// Return as []any for multi-select
	result := make([]any, len(paths))
	for i, p := range paths {
		result[i] = p
	}
	return result, nil
}

func (e *Evaluator) fnFileSave(args []any) (any, error) {
	if e.Native == nil {
		return nil, fmt.Errorf("fileSave: not available in headless mode")
	}
	title := "Save"
	defaultName := ""
	allowedTypes := ""
	if len(args) >= 1 {
		title = toString(args[0])
	}
	if len(args) >= 2 {
		defaultName = toString(args[1])
	}
	if len(args) >= 3 {
		allowedTypes = toString(args[2])
	}
	path, err := e.Native.FileSave(title, defaultName, allowedTypes)
	if err != nil {
		return nil, err
	}
	return path, nil
}

func (e *Evaluator) fnFileRead(args []any) (any, error) {
	if e.Native == nil {
		return nil, fmt.Errorf("fileRead: not available in headless mode")
	}
	if len(args) < 1 {
		return nil, fmt.Errorf("fileRead: requires path argument")
	}
	path := toString(args[0])
	content, err := e.Native.FileRead(path)
	if err != nil {
		return nil, err
	}
	return content, nil
}

func (e *Evaluator) fnFileWrite(args []any) (any, error) {
	if e.Native == nil {
		return nil, fmt.Errorf("fileWrite: not available in headless mode")
	}
	if len(args) < 2 {
		return nil, fmt.Errorf("fileWrite: requires path and content arguments")
	}
	path := toString(args[0])
	content := args[1]
	var s string
	switch v := content.(type) {
	case string:
		s = v
	default:
		b, err := json.Marshal(v)
		if err != nil {
			return nil, fmt.Errorf("fileWrite: failed to serialize content: %w", err)
		}
		s = string(b)
	}
	if err := e.Native.FileWrite(path, s); err != nil {
		return nil, err
	}
	return "ok", nil
}

func (e *Evaluator) fnFileAppend(args []any) (any, error) {
	if e.Native == nil {
		return nil, fmt.Errorf("fileAppend: not available in headless mode")
	}
	if len(args) < 2 {
		return nil, fmt.Errorf("fileAppend: requires path and content arguments")
	}
	path := toString(args[0])
	content := args[1]
	var s string
	switch v := content.(type) {
	case string:
		s = v
	default:
		b, err := json.Marshal(v)
		if err != nil {
			return nil, fmt.Errorf("fileAppend: failed to serialize content: %w", err)
		}
		s = string(b)
	}
	if err := e.Native.FileAppend(path, s); err != nil {
		return nil, err
	}
	return "ok", nil
}

func (e *Evaluator) fnAlert(args []any) (any, error) {
	if e.Native == nil {
		return nil, fmt.Errorf("alert: not available in headless mode")
	}
	if len(args) < 2 {
		return nil, fmt.Errorf("alert requires at least 2 args (title, message)")
	}
	title := toString(args[0])
	message := toString(args[1])
	style := "informational"
	var buttons []string
	if len(args) >= 3 {
		style = toString(args[2])
	}
	if len(args) >= 4 {
		if arr, ok := args[3].([]any); ok {
			for _, b := range arr {
				buttons = append(buttons, toString(b))
			}
		} else if s := toString(args[3]); s != "" {
			buttons = strings.Split(s, ",")
		}
	}
	idx, err := e.Native.Alert(title, message, style, buttons)
	if err != nil {
		return nil, err
	}
	return float64(idx), nil
}

func (e *Evaluator) fnHttpGet(args []any) (any, error) {
	if len(args) < 1 {
		return nil, fmt.Errorf("httpGet requires 1 arg (url)")
	}
	url := toString(args[0])
	resp, err := httpClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("httpGet: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 10*1024*1024))
	if err != nil {
		return nil, fmt.Errorf("httpGet: %w", err)
	}
	return string(body), nil
}

func (e *Evaluator) fnCameraCapture(args []any) (any, error) {
	if e.Native == nil {
		return nil, fmt.Errorf("cameraCapture: not available in headless mode")
	}
	devicePosition := "front"
	if len(args) >= 1 {
		devicePosition = toString(args[0])
	}
	path, err := e.Native.CameraCapture(devicePosition)
	if err != nil {
		return nil, err
	}
	return path, nil
}

func (e *Evaluator) fnAudioRecordStart(args []any) (any, error) {
	if e.Native == nil {
		return nil, fmt.Errorf("audioRecordStart: not available in headless mode")
	}
	format := "m4a"
	sampleRate := 44100.0
	channels := 1
	if len(args) >= 1 {
		format = toString(args[0])
	}
	if len(args) >= 2 {
		if v, err := toFloat(args[1]); err == nil {
			sampleRate = v
		}
	}
	if len(args) >= 3 {
		if v, err := toFloat(args[2]); err == nil {
			channels = int(v)
		}
	}
	id, err := e.Native.AudioRecordStart(format, sampleRate, channels)
	if err != nil {
		return nil, err
	}
	return id, nil
}

func (e *Evaluator) fnAudioRecordStop(args []any) (any, error) {
	if e.Native == nil {
		return nil, fmt.Errorf("audioRecordStop: not available in headless mode")
	}
	if len(args) < 1 {
		return nil, fmt.Errorf("audioRecordStop requires 1 arg (recordingID)")
	}
	path, err := e.Native.AudioRecordStop(toString(args[0]))
	if err != nil {
		return nil, err
	}
	return path, nil
}

func (e *Evaluator) fnScreenCapture(args []any) (any, error) {
	if e.Native == nil {
		return nil, fmt.Errorf("screenCapture: not available in headless mode")
	}
	captureType := "screen"
	if len(args) >= 1 {
		captureType = toString(args[0])
	}
	path, err := e.Native.ScreenCapture(captureType)
	if err != nil {
		return nil, err
	}
	return path, nil
}

func (e *Evaluator) fnScreenRecordStart(args []any) (any, error) {
	if e.Native == nil {
		return nil, fmt.Errorf("screenRecordStart: not available in headless mode")
	}
	captureType := "screen"
	if len(args) >= 1 {
		captureType = toString(args[0])
	}
	id, err := e.Native.ScreenRecordStart(captureType)
	if err != nil {
		return nil, err
	}
	return id, nil
}

func (e *Evaluator) fnScreenRecordStop(args []any) (any, error) {
	if e.Native == nil {
		return nil, fmt.Errorf("screenRecordStop: not available in headless mode")
	}
	if len(args) < 1 {
		return nil, fmt.Errorf("screenRecordStop requires 1 arg (recordingID)")
	}
	path, err := e.Native.ScreenRecordStop(toString(args[0]))
	if err != nil {
		return nil, err
	}
	return path, nil
}

func (e *Evaluator) fnHttpPost(args []any) (any, error) {
	if len(args) < 2 {
		return nil, fmt.Errorf("httpPost requires at least 2 args (url, body)")
	}
	url := toString(args[0])
	reqBody := toString(args[1])
	contentType := "application/json"
	if len(args) >= 3 {
		contentType = toString(args[2])
	}
	resp, err := httpClient.Post(url, contentType, strings.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("httpPost: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 10*1024*1024))
	if err != nil {
		return nil, fmt.Errorf("httpPost: %w", err)
	}
	return string(body), nil
}

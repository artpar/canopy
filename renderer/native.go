package renderer

// NativeProvider exposes platform-native capabilities to the engine evaluator.
// All methods are safe to call from any goroutine; implementations handle
// main-thread dispatch internally when required.
type NativeProvider interface {
	// Notify sends a macOS notification. Returns nil on success.
	Notify(title, body, subtitle string) error

	// ClipboardRead returns the current clipboard text.
	ClipboardRead() (string, error)

	// ClipboardWrite sets the clipboard text.
	ClipboardWrite(text string) error

	// OpenURL opens a URL or file path in the default application.
	OpenURL(url string) error

	// FileOpen shows an open panel and returns selected path(s).
	// allowedTypes is a comma-separated list of extensions (e.g. "txt,md,json").
	// Returns nil slice if user cancelled.
	FileOpen(title string, allowedTypes string, allowMultiple bool) ([]string, error)

	// FileSave shows a save panel and returns the selected path.
	// Returns empty string if user cancelled.
	FileSave(title string, defaultName string, allowedTypes string) (string, error)

	// Alert shows a modal alert and returns the clicked button index (0-based).
	// style: "informational", "warning", or "critical".
	// buttons: list of button titles (first is default). Empty uses ["OK"].
	Alert(title, message, style string, buttons []string) (int, error)

	// CameraCapture takes a one-shot photo without a preview component.
	// devicePosition: "front" or "back". Returns the file path to the JPEG.
	CameraCapture(devicePosition string) (string, error)

	// AudioRecordStart begins recording audio to a temp file.
	// format: "m4a" or "wav". Returns a recording ID for AudioRecordStop.
	AudioRecordStart(format string, sampleRate float64, channels int) (string, error)

	// AudioRecordStop stops a recording and returns the file path.
	AudioRecordStop(recordingID string) (string, error)

	// ScreenCapture takes a screenshot and returns the file path to the PNG.
	// captureType: "screen" (entire display).
	ScreenCapture(captureType string) (string, error)

	// ScreenRecordStart begins recording the screen.
	// Returns a recording ID for ScreenRecordStop.
	ScreenRecordStart(captureType string) (string, error)

	// ScreenRecordStop stops a screen recording and returns the video file path.
	ScreenRecordStop(recordingID string) (string, error)

	// FileRead reads a file and returns its contents as a string.
	FileRead(path string) (string, error)

	// FileWrite writes content to a file, creating or overwriting it.
	FileWrite(path string, content string) error

	// FileAppend appends content to a file, creating it if missing.
	FileAppend(path string, content string) error

	// CleanupAll stops all active recordings and releases resources.
	CleanupAll()
}

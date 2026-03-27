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
}

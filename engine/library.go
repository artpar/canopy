package engine

import (
	"encoding/json"
	"fmt"
	"canopy/jlog"
	"canopy/protocol"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Library manages a persistent collection of defineComponent definitions
// in ~/.canopy/library/. Components saved here are available across sessions.
type Library struct {
	dir  string
	defs map[string]*protocol.DefineComponent
}

// NewLibrary creates a library backed by ~/.canopy/library/.
func NewLibrary() *Library {
	home, _ := os.UserHomeDir()
	dir := filepath.Join(home, ".canopy", "library")
	return &Library{
		dir:  dir,
		defs: make(map[string]*protocol.DefineComponent),
	}
}

// Load reads all *.jsonl files from the library directory, parsing each
// as a DefineComponent. Errors on individual files are logged and skipped.
func (lib *Library) Load() {
	entries, err := os.ReadDir(lib.dir)
	if err != nil {
		// Directory may not exist yet — not an error
		return
	}
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".jsonl" {
			continue
		}
		path := filepath.Join(lib.dir, e.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			jlog.Warnf("library", "", "read error %s: %v", path, err)
			continue
		}
		var dc protocol.DefineComponent
		if err := json.Unmarshal(data, &dc); err != nil {
			jlog.Warnf("library", "", "parse error %s: %v", path, err)
			continue
		}
		if dc.Name == "" {
			jlog.Warnf("library", "", "skipping %s: no name", path)
			continue
		}
		lib.defs[dc.Name] = &dc
		jlog.Debugf("library", "", "loaded component %q from %s", dc.Name, e.Name())
	}
	if len(lib.defs) > 0 {
		jlog.Infof("library", "", "loaded %d components from %s", len(lib.defs), lib.dir)
	}
}

// Save writes a DefineComponent to the library directory as <name>.jsonl.
// Idempotent — overwrites if the component already exists.
func (lib *Library) Save(dc *protocol.DefineComponent) error {
	if dc.Name == "" {
		return fmt.Errorf("cannot save component with empty name")
	}
	if err := os.MkdirAll(lib.dir, 0755); err != nil {
		return fmt.Errorf("create library dir: %w", err)
	}
	data, err := json.Marshal(dc)
	if err != nil {
		return fmt.Errorf("marshal component %q: %w", dc.Name, err)
	}
	path := filepath.Join(lib.dir, dc.Name+".jsonl")
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	lib.defs[dc.Name] = dc
	jlog.Infof("library", "", "saved component %q to %s", dc.Name, path)
	return nil
}

// Defs returns the loaded component definitions.
func (lib *Library) Defs() map[string]*protocol.DefineComponent {
	return lib.defs
}

// ComponentListForPrompt returns a text block listing available library
// components with their parameters, suitable for inclusion in an LLM
// system prompt.
func (lib *Library) ComponentListForPrompt() string {
	if len(lib.defs) == 0 {
		return ""
	}
	names := make([]string, 0, len(lib.defs))
	for name := range lib.defs {
		names = append(names, name)
	}
	sort.Strings(names)

	var b strings.Builder
	b.WriteString("\nCOMPONENT LIBRARY (reusable via useComponent):\n")
	for _, name := range names {
		dc := lib.defs[name]
		params := "none"
		if len(dc.Params) > 0 {
			params = strings.Join(dc.Params, ", ")
		}
		fmt.Fprintf(&b, "- %s (params: %s)\n", name, params)
	}
	return b.String()
}

package engine

import (
	"encoding/json"
	"fmt"
	"os"
)

// FFIConfig is the top-level convention file describing native libraries and their functions.
type FFIConfig struct {
	Libraries []LibConfig `json:"libraries"`
}

// LibConfig declares a single native library and its exported functions.
type LibConfig struct {
	Path      string       `json:"path"`
	Prefix    string       `json:"prefix"`
	Functions []FuncConfig `json:"functions"`
}

// FuncConfig declares a single native function's evaluation-level signature.
// The actual C call is always: const char* symbol(const char* json_args)
type FuncConfig struct {
	Name   string `json:"name"`
	Symbol string `json:"symbol"`
}

// LoadFFIConfig reads and parses an FFI convention file.
func LoadFFIConfig(path string) (*FFIConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("ffi config: %w", err)
	}
	var cfg FFIConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("ffi config: %w", err)
	}
	return &cfg, nil
}

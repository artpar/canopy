package registry

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

// Registry manages the local package registry at ~/.canopy/registry.json.
type Registry struct {
	mu   sync.Mutex
	path string
	data RegistryFile
}

// New creates a registry backed by ~/.canopy/registry.json.
func New() (*Registry, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	return NewAt(filepath.Join(home, ".canopy"))
}

// NewAt creates a registry backed by dir/registry.json. Used for testing.
func NewAt(dir string) (*Registry, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}
	r := &Registry{
		path: filepath.Join(dir, "registry.json"),
		data: RegistryFile{Version: 1, Packages: make(map[string]*PackageEntry)},
	}
	r.load()
	return r, nil
}

func (r *Registry) load() {
	data, err := os.ReadFile(r.path)
	if err != nil {
		return
	}
	var rf RegistryFile
	if json.Unmarshal(data, &rf) == nil && rf.Packages != nil {
		r.data = rf
	}
}

func (r *Registry) save() error {
	data, err := json.MarshalIndent(r.data, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(r.path, data, 0644)
}

// Get returns an installed package entry by key (owner/name). Nil if not found.
func (r *Registry) Get(key string) *PackageEntry {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.data.Packages[key]
}

// Put saves or updates a package entry.
func (r *Registry) Put(key string, entry *PackageEntry) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.data.Packages[key] = entry
	return r.save()
}

// Remove deletes a package entry from the registry.
func (r *Registry) Remove(key string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.data.Packages, key)
	return r.save()
}

// List returns all installed packages, optionally filtered by type.
func (r *Registry) List(pkgType PackageType) []*PackageEntry {
	r.mu.Lock()
	defer r.mu.Unlock()
	var result []*PackageEntry
	for _, entry := range r.data.Packages {
		if pkgType != "" && entry.Type != pkgType {
			continue
		}
		result = append(result, entry)
	}
	return result
}

// AllEntries returns the full packages map.
func (r *Registry) AllEntries() map[string]*PackageEntry {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make(map[string]*PackageEntry, len(r.data.Packages))
	for k, v := range r.data.Packages {
		out[k] = v
	}
	return out
}

// AppsDir returns the path to ~/.canopy/apps/.
func AppsDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, ".canopy", "apps")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	return dir, nil
}

// ThemesDir returns ~/.canopy/themes/.
func ThemesDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, ".canopy", "themes")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	return dir, nil
}

// FFIDir returns ~/.canopy/ffi/.
func FFIDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, ".canopy", "ffi")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	return dir, nil
}

// LibraryDir returns ~/.canopy/library/.
func LibraryDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, ".canopy", "library")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	return dir, nil
}

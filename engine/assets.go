package engine

import "sync"

// AssetRegistry stores asset aliases and their resolved source paths.
type AssetRegistry struct {
	mu     sync.RWMutex
	assets map[string]assetEntry
}

type assetEntry struct {
	Kind string
	Src  string
}

func NewAssetRegistry() *AssetRegistry {
	return &AssetRegistry{
		assets: make(map[string]assetEntry),
	}
}

// Register adds or updates an asset alias.
func (r *AssetRegistry) Register(alias, kind, src string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.assets[alias] = assetEntry{Kind: kind, Src: src}
}

// Resolve returns the src for an alias, or empty string if not found.
func (r *AssetRegistry) Resolve(alias string) string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if e, ok := r.assets[alias]; ok {
		return e.Src
	}
	return ""
}

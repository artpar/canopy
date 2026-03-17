package engine

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// CacheKey computes a SHA256 hex digest covering the prompt content and the
// component reference (which includes the system prompt template and library
// manifest). This ensures the cache invalidates when either changes.
func CacheKey(prompt, componentRef string) string {
	h := sha256.New()
	h.Write([]byte(prompt))
	h.Write([]byte{0}) // separator
	h.Write([]byte(componentRef))
	return fmt.Sprintf("%x", h.Sum(nil))
}

// CachePathsForFile computes cache file paths adjacent to a prompt file.
// This keeps sample_apps caches co-located with their prompts.
// Returns (jsonlPath, hashPath, tmpPath).
func CachePathsForFile(promptFile string) (jsonl, hash, tmp string) {
	dir := filepath.Dir(promptFile)
	base := strings.TrimSuffix(filepath.Base(promptFile), filepath.Ext(promptFile))
	jsonl = filepath.Join(dir, base+".jsonl")
	hash = filepath.Join(dir, "."+base+".hash")
	tmp = filepath.Join(dir, base+".jsonl.tmp")
	return
}

// CachePathsForPrompt computes cache file paths in the central cache directory
// (~/.cache/jview/) for inline prompts (--prompt or --claude-code).
// Uses the first 16 hex chars of the hash as the filename.
// Returns (jsonlPath, hashPath, tmpPath).
func CachePathsForPrompt(prompt, componentRef string) (jsonl, hash, tmp string) {
	home, _ := os.UserHomeDir()
	dir := filepath.Join(home, ".cache", "jview")
	key := CacheKey(prompt, componentRef)
	prefix := key[:16]
	jsonl = filepath.Join(dir, prefix+".jsonl")
	hash = filepath.Join(dir, "."+prefix+".hash")
	tmp = filepath.Join(dir, prefix+".jsonl.tmp")
	return
}

// CacheValid returns true if the cached JSONL exists and its hash file
// matches the current prompt+componentRef hash.
func CacheValid(jsonlPath, hashPath, prompt, componentRef string) bool {
	// JSONL must exist
	if _, err := os.Stat(jsonlPath); err != nil {
		return false
	}
	// Hash file must exist and match
	stored, err := os.ReadFile(hashPath)
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(stored)) == CacheKey(prompt, componentRef)
}

// WriteCacheHash writes the hash of prompt+componentRef to the hash file.
func WriteCacheHash(hashPath, prompt, componentRef string) error {
	dir := filepath.Dir(hashPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	return os.WriteFile(hashPath, []byte(CacheKey(prompt, componentRef)+"\n"), 0644)
}

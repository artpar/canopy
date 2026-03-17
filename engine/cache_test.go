package engine

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCachePathsForFile(t *testing.T) {
	jsonl, hash, tmp := CachePathsForFile("/foo/bar/prompt.txt")
	if jsonl != "/foo/bar/prompt.jsonl" {
		t.Errorf("jsonl = %q, want /foo/bar/prompt.jsonl", jsonl)
	}
	if hash != "/foo/bar/.prompt.hash" {
		t.Errorf("hash = %q, want /foo/bar/.prompt.hash", hash)
	}
	if tmp != "/foo/bar/prompt.jsonl.tmp" {
		t.Errorf("tmp = %q, want /foo/bar/prompt.jsonl.tmp", tmp)
	}
}

func TestCachePathsForFileNestedDir(t *testing.T) {
	jsonl, hash, tmp := CachePathsForFile("sample_apps/calculator/prompt.txt")
	if jsonl != "sample_apps/calculator/prompt.jsonl" {
		t.Errorf("jsonl = %q", jsonl)
	}
	if hash != "sample_apps/calculator/.prompt.hash" {
		t.Errorf("hash = %q", hash)
	}
	if tmp != "sample_apps/calculator/prompt.jsonl.tmp" {
		t.Errorf("tmp = %q", tmp)
	}
}

func TestCacheKey(t *testing.T) {
	h1 := CacheKey("hello", "ref")
	h2 := CacheKey("hello", "ref")
	h3 := CacheKey("world", "ref")
	h4 := CacheKey("hello", "ref2")

	if h1 != h2 {
		t.Error("same inputs should produce same hash")
	}
	if h1 == h3 {
		t.Error("different prompt should produce different hash")
	}
	if h1 == h4 {
		t.Error("different componentRef should produce different hash")
	}
	if len(h1) != 64 {
		t.Errorf("hash length = %d, want 64 hex chars", len(h1))
	}
}

func TestCacheValidMissing(t *testing.T) {
	if CacheValid("/nonexistent/prompt.jsonl", "/nonexistent/.prompt.hash", "hello", "ref") {
		t.Error("expected false for nonexistent files")
	}
}

func TestCacheValidRoundTrip(t *testing.T) {
	dir := t.TempDir()
	promptFile := filepath.Join(dir, "prompt.txt")
	jsonlPath, hashPath, _ := CachePathsForFile(promptFile)

	prompt := "Build a calculator"
	componentRef := "system-prompt-v1"

	// Write prompt file (not needed for new API, just JSONL + hash)
	os.WriteFile(promptFile, []byte(prompt), 0644)

	// No cache yet
	if CacheValid(jsonlPath, hashPath, prompt, componentRef) {
		t.Error("expected false before cache exists")
	}

	// Write JSONL
	os.WriteFile(jsonlPath, []byte(`{"type":"createSurface"}`+"\n"), 0644)

	// No hash yet
	if CacheValid(jsonlPath, hashPath, prompt, componentRef) {
		t.Error("expected false before hash exists")
	}

	// Write hash
	if err := WriteCacheHash(hashPath, prompt, componentRef); err != nil {
		t.Fatal(err)
	}

	// Now valid
	if !CacheValid(jsonlPath, hashPath, prompt, componentRef) {
		t.Error("expected true after writing hash")
	}

	// Change prompt → invalid
	if CacheValid(jsonlPath, hashPath, "Build a todo app", componentRef) {
		t.Error("expected false after prompt changed")
	}

	// Change componentRef → invalid
	if CacheValid(jsonlPath, hashPath, prompt, "system-prompt-v2") {
		t.Error("expected false after componentRef changed")
	}
}

func TestCachePathsForPrompt(t *testing.T) {
	jsonl, hash, tmp := CachePathsForPrompt("Build a counter", "ref")
	// Should be in ~/.cache/jview/
	if filepath.Ext(jsonl) != ".jsonl" {
		t.Errorf("jsonl should end in .jsonl, got %q", jsonl)
	}
	if !filepath.IsAbs(jsonl) {
		t.Errorf("jsonl should be absolute, got %q", jsonl)
	}
	if filepath.Dir(jsonl) != filepath.Dir(hash) {
		t.Error("jsonl and hash should be in same directory")
	}
	if filepath.Dir(jsonl) != filepath.Dir(tmp) {
		t.Error("jsonl and tmp should be in same directory")
	}
}

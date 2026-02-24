package transport

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestCachePaths(t *testing.T) {
	jsonl, hash, tmp := CachePaths("/foo/bar/prompt.txt")
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

func TestCachePathsNestedDir(t *testing.T) {
	jsonl, hash, tmp := CachePaths("sample_apps/calculator/prompt.txt")
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

func TestPromptHash(t *testing.T) {
	h1 := PromptHash([]byte("hello"))
	h2 := PromptHash([]byte("hello"))
	h3 := PromptHash([]byte("world"))

	if h1 != h2 {
		t.Error("same content should produce same hash")
	}
	if h1 == h3 {
		t.Error("different content should produce different hash")
	}
	if len(h1) != 64 {
		t.Errorf("hash length = %d, want 64 hex chars", len(h1))
	}
}

func TestPromptHashIncludesSystemPrompt(t *testing.T) {
	content := []byte("Build a calculator")

	// A plain SHA256 of just the content must differ from PromptHash,
	// proving the system prompt is mixed into the hash.
	plain := sha256.Sum256(content)
	plainHex := fmt.Sprintf("%x", plain[:])

	got := PromptHash(content)
	if got == plainHex {
		t.Error("PromptHash should differ from plain SHA256 of content (system prompt not included)")
	}
}

func TestCacheValidMissing(t *testing.T) {
	if CacheValid("/nonexistent/prompt.txt") {
		t.Error("expected false for nonexistent file")
	}
}

func TestCacheValidRoundTrip(t *testing.T) {
	dir := t.TempDir()
	promptFile := filepath.Join(dir, "prompt.txt")
	jsonlPath, _, _ := CachePaths(promptFile)

	// Write prompt
	os.WriteFile(promptFile, []byte("Build a calculator"), 0644)

	// No cache yet
	if CacheValid(promptFile) {
		t.Error("expected false before cache exists")
	}

	// Write JSONL (content doesn't matter for validity check)
	os.WriteFile(jsonlPath, []byte(`{"type":"createSurface"}`+"\n"), 0644)

	// No hash yet
	if CacheValid(promptFile) {
		t.Error("expected false before hash exists")
	}

	// Write hash
	if err := WriteHashFile(promptFile); err != nil {
		t.Fatal(err)
	}

	// Now valid
	if !CacheValid(promptFile) {
		t.Error("expected true after writing hash")
	}

	// Change prompt → invalid
	os.WriteFile(promptFile, []byte("Build a todo app"), 0644)
	if CacheValid(promptFile) {
		t.Error("expected false after prompt changed")
	}
}

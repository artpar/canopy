package registry

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestRegistryCRUD(t *testing.T) {
	dir := t.TempDir()
	reg, err := NewAt(dir)
	if err != nil {
		t.Fatal(err)
	}

	// Empty initially
	if got := reg.Get("owner/pkg"); got != nil {
		t.Error("expected nil for missing key")
	}
	if got := reg.List(""); len(got) != 0 {
		t.Errorf("expected 0, got %d", len(got))
	}
	if got := reg.AllEntries(); len(got) != 0 {
		t.Errorf("expected 0, got %d", len(got))
	}

	// Put
	entry := &PackageEntry{
		Name:        "pkg",
		Type:        TypeApp,
		Repo:        "owner/pkg",
		Version:     "1.0.0",
		InstalledAt: time.Now().UTC(),
		Path:        "/tmp/pkg",
	}
	if err := reg.Put("owner/pkg", entry); err != nil {
		t.Fatal(err)
	}

	// Get
	got := reg.Get("owner/pkg")
	if got == nil {
		t.Fatal("expected entry")
	}
	if got.Version != "1.0.0" {
		t.Errorf("got %q", got.Version)
	}

	// List all
	all := reg.List("")
	if len(all) != 1 {
		t.Errorf("expected 1, got %d", len(all))
	}

	// List filtered
	if got := reg.List(TypeComponent); len(got) != 0 {
		t.Errorf("expected 0 for component filter, got %d", len(got))
	}
	if got := reg.List(TypeApp); len(got) != 1 {
		t.Errorf("expected 1 for app filter, got %d", len(got))
	}

	// AllEntries
	entries := reg.AllEntries()
	if len(entries) != 1 {
		t.Errorf("expected 1, got %d", len(entries))
	}

	// Remove
	if err := reg.Remove("owner/pkg"); err != nil {
		t.Fatal(err)
	}
	if got := reg.Get("owner/pkg"); got != nil {
		t.Error("expected nil after remove")
	}
}

func TestRegistryPersistence(t *testing.T) {
	dir := t.TempDir()

	// Create and populate
	reg, _ := NewAt(dir)
	reg.Put("owner/pkg", &PackageEntry{
		Name:    "pkg",
		Type:    TypeApp,
		Version: "2.0.0",
	})

	// Reload from disk
	reg2, _ := NewAt(dir)
	got := reg2.Get("owner/pkg")
	if got == nil {
		t.Fatal("expected entry after reload")
	}
	if got.Version != "2.0.0" {
		t.Errorf("got %q", got.Version)
	}
}

func TestRegistryLoadInvalid(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "registry.json"), []byte("not json"), 0644)

	reg, err := NewAt(dir)
	if err != nil {
		t.Fatal(err)
	}
	// Should have empty packages (failed to parse)
	if len(reg.AllEntries()) != 0 {
		t.Error("expected empty after invalid JSON")
	}
}

func TestRegistryLoadNullPackages(t *testing.T) {
	dir := t.TempDir()
	data, _ := json.Marshal(RegistryFile{Version: 1, Packages: nil})
	os.WriteFile(filepath.Join(dir, "registry.json"), data, 0644)

	reg, _ := NewAt(dir)
	// Should use default empty map
	if reg.AllEntries() == nil {
		t.Error("expected non-nil map")
	}
}

func TestNew(t *testing.T) {
	reg, err := New()
	if err != nil {
		t.Fatal(err)
	}
	if reg == nil {
		t.Error("expected non-nil registry")
	}
}

func TestDirHelpers(t *testing.T) {
	d, err := AppsDir()
	if err != nil {
		t.Fatal(err)
	}
	if d == "" {
		t.Error("empty apps dir")
	}

	d, err = ThemesDir()
	if err != nil {
		t.Fatal(err)
	}
	if d == "" {
		t.Error("empty themes dir")
	}

	d, err = FFIDir()
	if err != nil {
		t.Fatal(err)
	}
	if d == "" {
		t.Error("empty ffi dir")
	}

	d, err = LibraryDir()
	if err != nil {
		t.Fatal(err)
	}
	if d == "" {
		t.Error("empty library dir")
	}
}

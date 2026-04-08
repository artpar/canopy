package registry

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"os"
	"path/filepath"
	"testing"
)

func makeTarball(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)

	for name, content := range files {
		hdr := &tar.Header{
			Name:     "repo-abc123/" + name,
			Size:     int64(len(content)),
			Mode:     0644,
			Typeflag: tar.TypeReg,
		}
		tw.WriteHeader(hdr)
		tw.Write([]byte(content))
	}

	tw.Close()
	gw.Close()
	return buf.Bytes()
}

func makeTarballWithDir(t *testing.T, files map[string]string, dirs []string) []byte {
	t.Helper()
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)

	for _, d := range dirs {
		tw.WriteHeader(&tar.Header{
			Name:     "repo-abc123/" + d + "/",
			Typeflag: tar.TypeDir,
			Mode:     0755,
		})
	}
	for name, content := range files {
		hdr := &tar.Header{
			Name:     "repo-abc123/" + name,
			Size:     int64(len(content)),
			Mode:     0644,
			Typeflag: tar.TypeReg,
		}
		tw.WriteHeader(hdr)
		tw.Write([]byte(content))
	}

	tw.Close()
	gw.Close()
	return buf.Bytes()
}

func TestReadManifestFromTarball(t *testing.T) {
	manifest := `{"name":"test-app","version":"1.0.0","type":"app","entry":"prompt.jsonl"}`
	data := makeTarball(t, map[string]string{
		"canopy.json":  manifest,
		"prompt.jsonl": `{"type":"createSurface"}`,
	})

	m, err := readManifestFromTarball(data)
	if err != nil {
		t.Fatal(err)
	}
	if m.Name != "test-app" {
		t.Errorf("got %q", m.Name)
	}
	if m.Type != TypeApp {
		t.Errorf("got type %q", m.Type)
	}
}

func TestReadManifestFromTarballMissing(t *testing.T) {
	data := makeTarball(t, map[string]string{
		"prompt.jsonl": `{}`,
	})

	_, err := readManifestFromTarball(data)
	if err == nil {
		t.Error("expected error for missing canopy.json")
	}
}

func TestReadManifestFromTarballInvalid(t *testing.T) {
	data := makeTarball(t, map[string]string{
		"canopy.json": "not json",
	})

	_, err := readManifestFromTarball(data)
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestReadManifestFromTarballBadGzip(t *testing.T) {
	_, err := readManifestFromTarball([]byte("not gzip"))
	if err == nil {
		t.Error("expected error for invalid gzip")
	}
}

func TestExtractTarball(t *testing.T) {
	data := makeTarballWithDir(t, map[string]string{
		"canopy.json":         `{"name":"test"}`,
		"subdir/prompt.jsonl": `{"type":"createSurface"}`,
	}, []string{"subdir"})

	dest := t.TempDir()
	if err := extractTarball(data, dest); err != nil {
		t.Fatal(err)
	}

	// Check files exist
	if _, err := os.Stat(filepath.Join(dest, "canopy.json")); err != nil {
		t.Error("canopy.json not extracted")
	}
	if _, err := os.Stat(filepath.Join(dest, "subdir", "prompt.jsonl")); err != nil {
		t.Error("subdir/prompt.jsonl not extracted")
	}

	// Check content
	content, _ := os.ReadFile(filepath.Join(dest, "canopy.json"))
	if string(content) != `{"name":"test"}` {
		t.Errorf("got %q", string(content))
	}
}

func TestExtractTarballBadGzip(t *testing.T) {
	err := extractTarball([]byte("not gzip"), t.TempDir())
	if err == nil {
		t.Error("expected error")
	}
}

func TestExtractTarballTopLevelOnly(t *testing.T) {
	// Tarball with only the top-level dir entry (no files inside)
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)
	tw.WriteHeader(&tar.Header{
		Name:     "repo-abc123/",
		Typeflag: tar.TypeDir,
		Mode:     0755,
	})
	tw.Close()
	gw.Close()

	dest := t.TempDir()
	err := extractTarball(buf.Bytes(), dest)
	if err != nil {
		t.Fatal(err)
	}
}

func TestCopyFile(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src.txt")
	dst := filepath.Join(dir, "dst.txt")

	os.WriteFile(src, []byte("hello"), 0644)
	if err := copyFile(src, dst); err != nil {
		t.Fatal(err)
	}

	content, _ := os.ReadFile(dst)
	if string(content) != "hello" {
		t.Errorf("got %q", string(content))
	}
}

func TestCopyFileMissing(t *testing.T) {
	dir := t.TempDir()
	err := copyFile(filepath.Join(dir, "missing"), filepath.Join(dir, "dst"))
	if err == nil {
		t.Error("expected error")
	}
}

func TestInstallPathForType(t *testing.T) {
	tests := []struct {
		typ     PackageType
		repo    string
		wantErr bool
	}{
		{TypeApp, "owner/repo", false},
		{TypeComponent, "owner/comp", false},
		{TypeTheme, "owner/theme", false},
		{TypeFFIConfig, "owner/ffi", false},
		{PackageType("unknown"), "owner/repo", true},
		{TypeApp, "noslash", true},
	}
	for _, tt := range tests {
		m := &Manifest{Type: tt.typ}
		_, err := installPathForType(m, tt.repo)
		if (err != nil) != tt.wantErr {
			t.Errorf("installPathForType(%s, %q) error = %v, wantErr %v", tt.typ, tt.repo, err, tt.wantErr)
		}
	}
}

func TestUninstallNotInstalled(t *testing.T) {
	dir := t.TempDir()
	reg, _ := NewAt(dir)

	err := Uninstall(reg, "owner/missing")
	if err == nil {
		t.Error("expected error for uninstalling missing package")
	}
}

func TestUninstallApp(t *testing.T) {
	dir := t.TempDir()
	reg, _ := NewAt(dir)

	pkgDir := filepath.Join(dir, "apps", "owner", "pkg")
	os.MkdirAll(pkgDir, 0755)
	os.WriteFile(filepath.Join(pkgDir, "test.txt"), []byte("test"), 0644)

	reg.Put("owner/pkg", &PackageEntry{
		Name: "pkg",
		Type: TypeApp,
		Path: pkgDir,
	})

	if err := Uninstall(reg, "owner/pkg"); err != nil {
		t.Fatal(err)
	}

	// Dir should be removed
	if _, err := os.Stat(pkgDir); !os.IsNotExist(err) {
		t.Error("package dir should be removed")
	}

	// Registry should be empty
	if reg.Get("owner/pkg") != nil {
		t.Error("registry should be empty")
	}
}

func TestUninstallComponent(t *testing.T) {
	dir := t.TempDir()
	reg, _ := NewAt(dir)

	pkgDir := filepath.Join(dir, "apps", "owner", "comp")
	os.MkdirAll(pkgDir, 0755)

	reg.Put("owner/comp", &PackageEntry{
		Name: "comp",
		Type: TypeComponent,
		Path: pkgDir,
	})

	err := Uninstall(reg, "owner/comp")
	if err != nil {
		t.Fatal(err)
	}
}

func TestUninstallTheme(t *testing.T) {
	dir := t.TempDir()
	reg, _ := NewAt(dir)

	reg.Put("owner/theme", &PackageEntry{
		Name: "mytheme",
		Type: TypeTheme,
		Path: filepath.Join(dir, "theme"),
	})

	err := Uninstall(reg, "owner/theme")
	if err != nil {
		t.Fatal(err)
	}
}

func TestUninstallFFI(t *testing.T) {
	dir := t.TempDir()
	reg, _ := NewAt(dir)

	reg.Put("owner/ffi", &PackageEntry{
		Name: "myffi",
		Type: TypeFFIConfig,
		Path: filepath.Join(dir, "ffi"),
	})

	err := Uninstall(reg, "owner/ffi")
	if err != nil {
		t.Fatal(err)
	}
}

package registry

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"canopy/pkg/github"
)

func setupTestServer(t *testing.T, manifest *Manifest, tarFiles map[string]string) (*github.Client, *httptest.Server) {
	t.Helper()

	manifestJSON, _ := json.Marshal(manifest)
	tarball := makeTarball(t, tarFiles)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/repos/owner/test-app/tags":
			json.NewEncoder(w).Encode([]github.Tag{
				{Name: "v1.0.0", Commit: struct {
					SHA string `json:"sha"`
				}{SHA: "abc123"}},
				{Name: "v1.1.0", Commit: struct {
					SHA string `json:"sha"`
				}{SHA: "def456"}},
			})
		case r.URL.Path == "/repos/owner/test-app/tarball/v1.1.0":
			w.Write(tarball)
		case r.URL.Path == "/repos/owner/test-app" && r.Method == "GET":
			json.NewEncoder(w).Encode(github.Repo{FullName: "owner/test-app", DefaultBranch: "main"})
		case r.URL.Path == "/repos/owner/test-app/git/ref/heads/main":
			json.NewEncoder(w).Encode(github.GitRef{Object: struct {
				SHA string `json:"sha"`
			}{SHA: "headsha"}})
		case r.URL.Path == "/repos/owner/test-app/git/refs" && r.Method == "POST":
			w.WriteHeader(201)
		case r.URL.Path == "/repos/owner/test-app/releases" && r.Method == "POST":
			json.NewEncoder(w).Encode(github.Release{TagName: "v1.0.0", HTMLURL: "https://release-url"})
		case r.URL.Path == "/repos/owner/test-app/topics" && r.Method == "GET":
			json.NewEncoder(w).Encode(struct {
				Names []string `json:"names"`
			}{Names: []string{"go"}})
		case r.URL.Path == "/repos/owner/test-app/topics" && r.Method == "PUT":
			w.WriteHeader(200)
		case r.URL.Path == "/repos/owner/test-app/contents/canopy.json":
			content := json.RawMessage(manifestJSON)
			json.NewEncoder(w).Encode(github.FileContent{Content: string(content), Encoding: ""})
		default:
			w.WriteHeader(404)
			w.Write([]byte(`{"message":"not found: ` + r.URL.Path + `"}`))
		}
	}))
	t.Cleanup(srv.Close)

	client := github.NewTestClient("test-token", srv.URL)
	return client, srv
}

func TestInstallApp(t *testing.T) {
	manifest := &Manifest{
		Name:    "test-app",
		Version: "1.1.0",
		Type:    TypeApp,
		Entry:   "prompt.jsonl",
	}
	manifestJSON, _ := json.Marshal(manifest)

	client, _ := setupTestServer(t, manifest, map[string]string{
		"canopy.json":  string(manifestJSON),
		"prompt.jsonl": `{"type":"createSurface"}`,
	})

	regDir := t.TempDir()
	reg, _ := NewAt(regDir)

	entry, err := Install(reg, client, "owner/test-app", "")
	if err != nil {
		t.Fatal(err)
	}
	if entry.Name != "test-app" {
		t.Errorf("got name %q", entry.Name)
	}
	if entry.Version != "1.1.0" {
		t.Errorf("got version %q", entry.Version)
	}
	if entry.Type != TypeApp {
		t.Errorf("got type %q", entry.Type)
	}

	// Verify installed on disk
	jsonlPath := filepath.Join(entry.Path, "prompt.jsonl")
	if _, err := os.Stat(jsonlPath); err != nil {
		t.Errorf("prompt.jsonl not found at %s", jsonlPath)
	}

	// Verify registry updated
	got := reg.Get("owner/test-app")
	if got == nil {
		t.Fatal("not in registry")
	}
	if got.Commit != "def456" {
		t.Errorf("got commit %q", got.Commit)
	}
}

func TestInstallWithVersion(t *testing.T) {
	manifest := &Manifest{
		Name:    "test-app",
		Version: "1.0.0",
		Type:    TypeApp,
		Entry:   "prompt.jsonl",
	}

	// Server returns v1.0.0 tarball at the matching path
	manifestJSON, _ := json.Marshal(manifest)
	tarball := makeTarball(t, map[string]string{
		"canopy.json":  string(manifestJSON),
		"prompt.jsonl": `{}`,
	})

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/repos/owner/test-app/tags":
			json.NewEncoder(w).Encode([]github.Tag{
				{Name: "v1.0.0", Commit: struct {
					SHA string `json:"sha"`
				}{SHA: "aaa"}},
				{Name: "v1.1.0", Commit: struct {
					SHA string `json:"sha"`
				}{SHA: "bbb"}},
			})
		case r.URL.Path == "/repos/owner/test-app/tarball/v1.0.0":
			w.Write(tarball)
		default:
			w.WriteHeader(404)
		}
	}))
	defer srv.Close()

	client := github.NewTestClient("", srv.URL)
	reg, _ := NewAt(t.TempDir())

	entry, err := Install(reg, client, "owner/test-app", "1.0.0")
	if err != nil {
		t.Fatal(err)
	}
	if entry.Version != "1.0.0" {
		t.Errorf("got %q", entry.Version)
	}
}

func TestInstallNoTags(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode([]github.Tag{})
	}))
	defer srv.Close()

	client := github.NewTestClient("", srv.URL)
	reg, _ := NewAt(t.TempDir())

	_, err := Install(reg, client, "owner/test-app", "")
	if err == nil {
		t.Error("expected error")
	}
}

func TestInstallNoMatchingVersion(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode([]github.Tag{
			{Name: "v1.0.0"},
		})
	}))
	defer srv.Close()

	client := github.NewTestClient("", srv.URL)
	reg, _ := NewAt(t.TempDir())

	_, err := Install(reg, client, "owner/test-app", ">=2.0.0")
	if err == nil {
		t.Error("expected error")
	}
}

func TestInstallComponent(t *testing.T) {
	manifest := &Manifest{
		Name:    "my-comp",
		Version: "1.0.0",
		Type:    TypeComponent,
		Entry:   "component.jsonl",
	}
	manifestJSON, _ := json.Marshal(manifest)
	tarball := makeTarball(t, map[string]string{
		"canopy.json":     string(manifestJSON),
		"component.jsonl": `{"type":"defineComponent"}`,
	})

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/repos/owner/comp/tags":
			json.NewEncoder(w).Encode([]github.Tag{
				{Name: "v1.0.0", Commit: struct {
					SHA string `json:"sha"`
				}{SHA: "c1"}},
			})
		case r.URL.Path == "/repos/owner/comp/tarball/v1.0.0":
			w.Write(tarball)
		default:
			w.WriteHeader(404)
		}
	}))
	defer srv.Close()

	client := github.NewTestClient("", srv.URL)
	reg, _ := NewAt(t.TempDir())

	entry, err := Install(reg, client, "owner/comp", "")
	if err != nil {
		t.Fatal(err)
	}
	if entry.Type != TypeComponent {
		t.Errorf("got type %q", entry.Type)
	}
}

func TestInstallTheme(t *testing.T) {
	manifest := &Manifest{
		Name:    "dark",
		Version: "1.0.0",
		Type:    TypeTheme,
		Entry:   "theme.jsonl",
	}
	manifestJSON, _ := json.Marshal(manifest)
	tarball := makeTarball(t, map[string]string{
		"canopy.json": string(manifestJSON),
		"theme.jsonl": `{"type":"setTheme"}`,
	})

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/repos/owner/theme/tags":
			json.NewEncoder(w).Encode([]github.Tag{
				{Name: "v1.0.0", Commit: struct {
					SHA string `json:"sha"`
				}{SHA: "t1"}},
			})
		case r.URL.Path == "/repos/owner/theme/tarball/v1.0.0":
			w.Write(tarball)
		default:
			w.WriteHeader(404)
		}
	}))
	defer srv.Close()

	client := github.NewTestClient("", srv.URL)
	reg, _ := NewAt(t.TempDir())

	entry, err := Install(reg, client, "owner/theme", "")
	if err != nil {
		t.Fatal(err)
	}
	if entry.Type != TypeTheme {
		t.Errorf("got type %q", entry.Type)
	}
}

func TestInstallFFI(t *testing.T) {
	manifest := &Manifest{
		Name:    "myffi",
		Version: "1.0.0",
		Type:    TypeFFIConfig,
		Entry:   "ffi.json",
	}
	manifestJSON, _ := json.Marshal(manifest)
	tarball := makeTarball(t, map[string]string{
		"canopy.json": string(manifestJSON),
		"ffi.json":    `{"libraries":[]}`,
	})

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/repos/owner/ffi/tags":
			json.NewEncoder(w).Encode([]github.Tag{
				{Name: "v1.0.0", Commit: struct {
					SHA string `json:"sha"`
				}{SHA: "f1"}},
			})
		case r.URL.Path == "/repos/owner/ffi/tarball/v1.0.0":
			w.Write(tarball)
		default:
			w.WriteHeader(404)
		}
	}))
	defer srv.Close()

	client := github.NewTestClient("", srv.URL)
	reg, _ := NewAt(t.TempDir())

	entry, err := Install(reg, client, "owner/ffi", "")
	if err != nil {
		t.Fatal(err)
	}
	if entry.Type != TypeFFIConfig {
		t.Errorf("got type %q", entry.Type)
	}
}

func TestCheckUpdates(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode([]github.Tag{
			{Name: "v2.0.0", Commit: struct {
				SHA string `json:"sha"`
			}{SHA: "new"}},
			{Name: "v1.0.0"},
		})
	}))
	defer srv.Close()

	client := github.NewTestClient("", srv.URL)
	reg, _ := NewAt(t.TempDir())
	reg.Put("owner/pkg", &PackageEntry{
		Name:    "pkg",
		Type:    TypeApp,
		Repo:    "owner/pkg",
		Version: "1.0.0",
	})

	updates, err := CheckUpdates(reg, client, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(updates) != 1 {
		t.Fatalf("expected 1 update, got %d", len(updates))
	}
	if updates[0].LatestVersion != "2.0.0" {
		t.Errorf("got %q", updates[0].LatestVersion)
	}
}

func TestCheckUpdatesNoUpdate(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode([]github.Tag{
			{Name: "v1.0.0"},
		})
	}))
	defer srv.Close()

	client := github.NewTestClient("", srv.URL)
	reg, _ := NewAt(t.TempDir())
	reg.Put("owner/pkg", &PackageEntry{
		Version: "1.0.0",
		Repo:    "owner/pkg",
	})

	updates, _ := CheckUpdates(reg, client, "")
	if len(updates) != 0 {
		t.Errorf("expected 0 updates, got %d", len(updates))
	}
}

func TestCheckUpdatesFilter(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode([]github.Tag{{Name: "v2.0.0"}})
	}))
	defer srv.Close()

	client := github.NewTestClient("", srv.URL)
	reg, _ := NewAt(t.TempDir())
	reg.Put("owner/a", &PackageEntry{Version: "1.0.0", Repo: "owner/a"})
	reg.Put("owner/b", &PackageEntry{Version: "1.0.0", Repo: "owner/b"})

	updates, _ := CheckUpdates(reg, client, "owner/a")
	if len(updates) != 1 {
		t.Errorf("expected 1, got %d", len(updates))
	}
}

func TestUpdate(t *testing.T) {
	manifest := &Manifest{
		Name:    "pkg",
		Version: "2.0.0",
		Type:    TypeApp,
		Entry:   "prompt.jsonl",
	}
	manifestJSON, _ := json.Marshal(manifest)
	tarball := makeTarball(t, map[string]string{
		"canopy.json":  string(manifestJSON),
		"prompt.jsonl": `{}`,
	})

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/repos/owner/pkg/tags":
			json.NewEncoder(w).Encode([]github.Tag{
				{Name: "v2.0.0", Commit: struct {
					SHA string `json:"sha"`
				}{SHA: "new"}},
				{Name: "v1.0.0"},
			})
		case r.URL.Path == "/repos/owner/pkg/tarball/v2.0.0":
			w.Write(tarball)
		default:
			w.WriteHeader(404)
		}
	}))
	defer srv.Close()

	client := github.NewTestClient("", srv.URL)
	reg, _ := NewAt(t.TempDir())
	reg.Put("owner/pkg", &PackageEntry{
		Name:    "pkg",
		Type:    TypeApp,
		Repo:    "owner/pkg",
		Version: "1.0.0",
	})

	updated, err := Update(reg, client, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(updated) != 1 {
		t.Fatalf("expected 1 update, got %d", len(updated))
	}
	if updated[0].LatestVersion != "2.0.0" {
		t.Errorf("got %q", updated[0].LatestVersion)
	}

	// Registry should be updated
	entry := reg.Get("owner/pkg")
	if entry == nil {
		t.Fatal("entry missing")
	}
	if entry.Version != "2.0.0" {
		t.Errorf("registry version %q", entry.Version)
	}
}

func TestUpdateNoUpdates(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode([]github.Tag{{Name: "v1.0.0"}})
	}))
	defer srv.Close()

	client := github.NewTestClient("", srv.URL)
	reg, _ := NewAt(t.TempDir())
	reg.Put("owner/pkg", &PackageEntry{Version: "1.0.0", Repo: "owner/pkg"})

	updated, err := Update(reg, client, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(updated) != 0 {
		t.Errorf("expected no updates, got %d", len(updated))
	}
}

func TestInstallTagListError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
	}))
	defer srv.Close()

	client := github.NewTestClient("", srv.URL)
	reg, _ := NewAt(t.TempDir())

	_, err := Install(reg, client, "owner/pkg", "")
	if err == nil {
		t.Error("expected error")
	}
}

func TestInstallDownloadError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/repos/owner/pkg/tags":
			json.NewEncoder(w).Encode([]github.Tag{
				{Name: "v1.0.0", Commit: struct {
					SHA string `json:"sha"`
				}{SHA: "abc"}},
			})
		default:
			w.WriteHeader(500)
		}
	}))
	defer srv.Close()

	client := github.NewTestClient("", srv.URL)
	reg, _ := NewAt(t.TempDir())

	_, err := Install(reg, client, "owner/pkg", "")
	if err == nil {
		t.Error("expected error for download failure")
	}
}

func TestInstallBadTarball(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/repos/owner/pkg/tags":
			json.NewEncoder(w).Encode([]github.Tag{
				{Name: "v1.0.0", Commit: struct {
					SHA string `json:"sha"`
				}{SHA: "abc"}},
			})
		case r.URL.Path == "/repos/owner/pkg/tarball/v1.0.0":
			w.Write([]byte("not a tarball"))
		}
	}))
	defer srv.Close()

	client := github.NewTestClient("", srv.URL)
	reg, _ := NewAt(t.TempDir())

	_, err := Install(reg, client, "owner/pkg", "")
	if err == nil {
		t.Error("expected error for bad tarball")
	}
}

func TestCheckUpdatesTagError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
	}))
	defer srv.Close()

	client := github.NewTestClient("", srv.URL)
	reg, _ := NewAt(t.TempDir())
	reg.Put("owner/pkg", &PackageEntry{Version: "1.0.0", Repo: "owner/pkg"})

	// Should not error, just skip the package
	updates, err := CheckUpdates(reg, client, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(updates) != 0 {
		t.Errorf("expected 0, got %d", len(updates))
	}
}

func TestCheckUpdatesNoTags(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode([]github.Tag{})
	}))
	defer srv.Close()

	client := github.NewTestClient("", srv.URL)
	reg, _ := NewAt(t.TempDir())
	reg.Put("owner/pkg", &PackageEntry{Version: "1.0.0", Repo: "owner/pkg"})

	updates, _ := CheckUpdates(reg, client, "")
	if len(updates) != 0 {
		t.Errorf("expected 0, got %d", len(updates))
	}
}

func TestCheckUpdatesBadVersion(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode([]github.Tag{{Name: "v2.0.0"}})
	}))
	defer srv.Close()

	client := github.NewTestClient("", srv.URL)
	reg, _ := NewAt(t.TempDir())
	reg.Put("owner/pkg", &PackageEntry{Version: "bad", Repo: "owner/pkg"})

	updates, _ := CheckUpdates(reg, client, "")
	if len(updates) != 0 {
		t.Errorf("expected 0, got %d", len(updates))
	}
}

func TestPublish(t *testing.T) {
	manifest := &Manifest{
		Name:        "test-app",
		Version:     "1.0.0",
		Type:        TypeApp,
		Entry:       "prompt.jsonl",
		Description: "A test app",
	}

	client, _ := setupTestServer(t, manifest, nil)

	dir := t.TempDir()
	manifestJSON, _ := json.Marshal(manifest)
	os.WriteFile(filepath.Join(dir, "canopy.json"), manifestJSON, 0644)

	result, err := Publish(client, dir, "owner/test-app", "")
	if err != nil {
		t.Fatal(err)
	}
	if result.Tag != "v1.0.0" {
		t.Errorf("got tag %q", result.Tag)
	}
	if result.ReleaseURL != "https://release-url" {
		t.Errorf("got url %q", result.ReleaseURL)
	}
}

func TestPublishWithTagOverride(t *testing.T) {
	manifest := &Manifest{
		Name:    "test-app",
		Version: "1.0.0",
		Type:    TypeApp,
		Entry:   "prompt.jsonl",
	}

	client, _ := setupTestServer(t, manifest, nil)

	dir := t.TempDir()
	manifestJSON, _ := json.Marshal(manifest)
	os.WriteFile(filepath.Join(dir, "canopy.json"), manifestJSON, 0644)

	result, err := Publish(client, dir, "owner/test-app", "2.0.0")
	if err != nil {
		t.Fatal(err)
	}
	if result.Tag != "v2.0.0" {
		t.Errorf("got tag %q", result.Tag)
	}
}

func TestPublishWithVPrefix(t *testing.T) {
	manifest := &Manifest{
		Name:    "test-app",
		Version: "1.0.0",
		Type:    TypeApp,
		Entry:   "prompt.jsonl",
	}

	client, _ := setupTestServer(t, manifest, nil)

	dir := t.TempDir()
	manifestJSON, _ := json.Marshal(manifest)
	os.WriteFile(filepath.Join(dir, "canopy.json"), manifestJSON, 0644)

	result, err := Publish(client, dir, "owner/test-app", "v3.0.0")
	if err != nil {
		t.Fatal(err)
	}
	if result.Tag != "v3.0.0" {
		t.Errorf("got tag %q", result.Tag)
	}
}

func TestPublishNoAuth(t *testing.T) {
	client := github.NewTestClient("", "http://localhost")
	_, err := Publish(client, ".", "owner/repo", "")
	if err == nil {
		t.Error("expected error")
	}
}

func TestPublishNoRepo(t *testing.T) {
	manifest := &Manifest{
		Name:    "test-app",
		Version: "1.0.0",
		Type:    TypeApp,
		Entry:   "prompt.jsonl",
	}

	dir := t.TempDir()
	manifestJSON, _ := json.Marshal(manifest)
	os.WriteFile(filepath.Join(dir, "canopy.json"), manifestJSON, 0644)

	client := github.NewTestClient("tok", "http://localhost")
	_, err := Publish(client, dir, "", "")
	if err == nil {
		t.Error("expected error for empty repo")
	}
}

func TestPublishNoManifest(t *testing.T) {
	client := github.NewTestClient("tok", "http://localhost")
	_, err := Publish(client, t.TempDir(), "owner/repo", "")
	if err == nil {
		t.Error("expected error for missing canopy.json")
	}
}

func TestPublishInvalidManifest(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "canopy.json"), []byte("not json"), 0644)

	client := github.NewTestClient("tok", "http://localhost")
	_, err := Publish(client, dir, "owner/repo", "")
	if err == nil {
		t.Error("expected error")
	}
}

func TestPublishBadManifest(t *testing.T) {
	dir := t.TempDir()
	m, _ := json.Marshal(Manifest{Name: "test"}) // missing required fields
	os.WriteFile(filepath.Join(dir, "canopy.json"), m, 0644)

	client := github.NewTestClient("tok", "http://localhost")
	_, err := Publish(client, dir, "owner/repo", "")
	if err == nil {
		t.Error("expected error for invalid manifest")
	}
}

func TestPublishNoDescription(t *testing.T) {
	manifest := &Manifest{
		Name:    "test-app",
		Version: "1.0.0",
		Type:    TypeApp,
		Entry:   "prompt.jsonl",
	}

	client, _ := setupTestServer(t, manifest, nil)

	dir := t.TempDir()
	manifestJSON, _ := json.Marshal(manifest)
	os.WriteFile(filepath.Join(dir, "canopy.json"), manifestJSON, 0644)

	result, err := Publish(client, dir, "owner/test-app", "")
	if err != nil {
		t.Fatal(err)
	}
	if result.Tag != "v1.0.0" {
		t.Errorf("got tag %q", result.Tag)
	}
}

package registry

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"canopy/pkg/github"
)

// Install downloads and installs a package from GitHub.
// If version is empty, the latest semver tag is used.
func Install(reg *Registry, client *github.Client, ownerRepo string, version string) (*PackageEntry, error) {
	// Resolve version to a tag
	tags, err := client.ListTags(ownerRepo)
	if err != nil {
		return nil, fmt.Errorf("list tags: %w", err)
	}

	tagNames := make([]string, len(tags))
	tagCommits := make(map[string]string)
	for i, t := range tags {
		tagNames[i] = t.Name
		tagCommits[t.Name] = t.Commit.SHA
	}

	tag, _, _ := FindBestTag(tagNames, version)
	if tag == "" {
		if version != "" {
			return nil, fmt.Errorf("no tag matching %q in %s", version, ownerRepo)
		}
		return nil, fmt.Errorf("no semver tags found in %s", ownerRepo)
	}
	commit := tagCommits[tag]

	// Download tarball
	tarData, err := client.DownloadTarball(ownerRepo, tag)
	if err != nil {
		return nil, fmt.Errorf("download: %w", err)
	}

	// Read canopy.json from tarball to determine type
	manifest, err := readManifestFromTarball(tarData)
	if err != nil {
		return nil, fmt.Errorf("read manifest: %w", err)
	}

	// Determine install path by type
	installDir, err := installPathForType(manifest, ownerRepo)
	if err != nil {
		return nil, err
	}

	// Clear any existing install
	os.RemoveAll(installDir)
	if err := os.MkdirAll(installDir, 0755); err != nil {
		return nil, err
	}

	// Extract tarball
	if err := extractTarball(tarData, installDir); err != nil {
		return nil, fmt.Errorf("extract: %w", err)
	}

	// For component packages, copy entry to library
	if manifest.Type == TypeComponent {
		if err := copyToLibrary(installDir, manifest); err != nil {
			return nil, fmt.Errorf("copy to library: %w", err)
		}
	}

	// For theme packages, copy entry to themes dir
	if manifest.Type == TypeTheme {
		if err := copyToThemes(installDir, manifest); err != nil {
			return nil, fmt.Errorf("copy to themes: %w", err)
		}
	}

	// For ffi-config packages, copy entry to ffi dir
	if manifest.Type == TypeFFIConfig {
		if err := copyToFFI(installDir, manifest); err != nil {
			return nil, fmt.Errorf("copy to ffi: %w", err)
		}
	}

	parsedVer, _ := ParseSemver(tag)
	entry := &PackageEntry{
		Name:        manifest.Name,
		Type:        manifest.Type,
		Repo:        ownerRepo,
		Version:     parsedVer.String(),
		Commit:      commit,
		InstalledAt: time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
		Path:        installDir,
		Entry:       manifest.Entry,
	}

	key := ownerRepo
	if err := reg.Put(key, entry); err != nil {
		return entry, fmt.Errorf("registry update: %w", err)
	}

	return entry, nil
}

// Uninstall removes an installed package.
func Uninstall(reg *Registry, key string) error {
	entry := reg.Get(key)
	if entry == nil {
		return fmt.Errorf("package %q not installed", key)
	}

	// Remove files
	if entry.Path != "" {
		os.RemoveAll(entry.Path)
	}

	// For component packages, also remove from library
	if entry.Type == TypeComponent {
		libDir, err := LibraryDir()
		if err == nil {
			os.Remove(filepath.Join(libDir, entry.Name+".jsonl"))
		}
	}
	if entry.Type == TypeTheme {
		dir, err := ThemesDir()
		if err == nil {
			os.Remove(filepath.Join(dir, entry.Name+".jsonl"))
		}
	}
	if entry.Type == TypeFFIConfig {
		dir, err := FFIDir()
		if err == nil {
			os.Remove(filepath.Join(dir, entry.Name+".json"))
		}
	}

	return reg.Remove(key)
}

// CheckUpdates checks if newer versions are available for installed packages.
func CheckUpdates(reg *Registry, client *github.Client, name string) ([]UpdateInfo, error) {
	entries := reg.AllEntries()
	var updates []UpdateInfo

	for key, entry := range entries {
		if name != "" && key != name {
			continue
		}

		tags, err := client.ListTags(entry.Repo)
		if err != nil {
			continue
		}
		tagNames := make([]string, len(tags))
		for i, t := range tags {
			tagNames[i] = t.Name
		}

		latest, _, _ := FindBestTag(tagNames, "")
		if latest == "" {
			continue
		}

		currentVer, err := ParseSemver(entry.Version)
		if err != nil {
			continue
		}
		latestVer, err := ParseSemver(latest)
		if err != nil {
			continue
		}

		if latestVer.Compare(currentVer) > 0 {
			updates = append(updates, UpdateInfo{
				Name:           key,
				CurrentVersion: entry.Version,
				LatestVersion:  latestVer.String(),
				Repo:           entry.Repo,
			})
		}
	}
	return updates, nil
}

// Update updates a single package or all packages if name is empty.
func Update(reg *Registry, client *github.Client, name string) ([]UpdateInfo, error) {
	updates, err := CheckUpdates(reg, client, name)
	if err != nil {
		return nil, err
	}

	var applied []UpdateInfo
	for _, u := range updates {
		_, err := Install(reg, client, u.Repo, "")
		if err != nil {
			continue
		}
		applied = append(applied, u)
	}
	return applied, nil
}

func installPathForType(m *Manifest, ownerRepo string) (string, error) {
	parts := strings.SplitN(ownerRepo, "/", 2)
	if len(parts) != 2 {
		return "", fmt.Errorf("invalid repo format: %q", ownerRepo)
	}
	owner, name := parts[0], parts[1]

	switch m.Type {
	case TypeApp:
		appsDir, err := AppsDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(appsDir, owner, name), nil
	case TypeComponent:
		appsDir, err := AppsDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(appsDir, owner, name), nil
	case TypeTheme:
		appsDir, err := AppsDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(appsDir, owner, name), nil
	case TypeFFIConfig:
		appsDir, err := AppsDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(appsDir, owner, name), nil
	default:
		return "", fmt.Errorf("unknown package type: %q", m.Type)
	}
}

func readManifestFromTarball(data []byte) (*Manifest, error) {
	gz, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer gz.Close()
	tr := tar.NewReader(gz)

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		// Tarball has a top-level directory prefix; match canopy.json at depth 1
		name := hdr.Name
		parts := strings.SplitN(name, "/", 2)
		if len(parts) == 2 && parts[1] == "canopy.json" {
			content, err := io.ReadAll(tr)
			if err != nil {
				return nil, err
			}
			var m Manifest
			if err := json.Unmarshal(content, &m); err != nil {
				return nil, fmt.Errorf("parse canopy.json: %w", err)
			}
			return &m, nil
		}
	}
	return nil, fmt.Errorf("canopy.json not found in tarball")
}

func extractTarball(data []byte, destDir string) error {
	gz, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return err
	}
	defer gz.Close()
	tr := tar.NewReader(gz)

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		// Strip top-level directory prefix
		name := hdr.Name
		parts := strings.SplitN(name, "/", 2)
		if len(parts) < 2 || parts[1] == "" {
			continue
		}
		relPath := parts[1]

		target := filepath.Join(destDir, relPath)

		// Prevent path traversal
		if !strings.HasPrefix(target, destDir+string(filepath.Separator)) && target != destDir {
			continue
		}

		switch hdr.Typeflag {
		case tar.TypeDir:
			os.MkdirAll(target, 0755)
		case tar.TypeReg:
			os.MkdirAll(filepath.Dir(target), 0755)
			f, err := os.Create(target)
			if err != nil {
				return err
			}
			if _, err := io.Copy(f, tr); err != nil {
				f.Close()
				return err
			}
			f.Close()
			os.Chmod(target, os.FileMode(hdr.Mode))
		}
	}
	return nil
}

func copyToLibrary(installDir string, m *Manifest) error {
	libDir, err := LibraryDir()
	if err != nil {
		return err
	}
	src := filepath.Join(installDir, m.Entry)
	dst := filepath.Join(libDir, m.Name+".jsonl")
	return copyFile(src, dst)
}

func copyToThemes(installDir string, m *Manifest) error {
	dir, err := ThemesDir()
	if err != nil {
		return err
	}
	src := filepath.Join(installDir, m.Entry)
	dst := filepath.Join(dir, m.Name+".jsonl")
	return copyFile(src, dst)
}

func copyToFFI(installDir string, m *Manifest) error {
	dir, err := FFIDir()
	if err != nil {
		return err
	}
	src := filepath.Join(installDir, m.Entry)
	dst := filepath.Join(dir, m.Name+".json")
	return copyFile(src, dst)
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}

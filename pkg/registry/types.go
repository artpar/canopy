package registry

import "time"

// PackageType is the kind of Canopy package.
type PackageType string

const (
	TypeApp       PackageType = "app"
	TypeComponent PackageType = "component"
	TypeTheme     PackageType = "theme"
	TypeFFIConfig PackageType = "ffi-config"
)

// Manifest is the canopy.json schema at the root of a package repo.
type Manifest struct {
	Name         string            `json:"name"`
	Version      string            `json:"version"`
	Type         PackageType       `json:"type"`
	Description  string            `json:"description,omitempty"`
	Author       string            `json:"author,omitempty"`
	License      string            `json:"license,omitempty"`
	Icon         string            `json:"icon,omitempty"`
	Entry        string            `json:"entry"`
	Prompt       string            `json:"prompt,omitempty"`
	Keywords     []string          `json:"keywords,omitempty"`
	Dependencies map[string]string `json:"dependencies,omitempty"`
}

// PackageEntry tracks an installed package in the local registry.
type PackageEntry struct {
	Name        string      `json:"name"`
	Type        PackageType `json:"type"`
	Repo        string      `json:"repo"`
	Version     string      `json:"version"`
	Commit      string      `json:"commit"`
	InstalledAt time.Time   `json:"installed_at"`
	UpdatedAt   time.Time   `json:"updated_at"`
	Path        string      `json:"path"`
	Entry       string      `json:"entry"`
}

// RegistryFile is the on-disk format of ~/.canopy/registry.json.
type RegistryFile struct {
	Version  int                      `json:"version"`
	Packages map[string]*PackageEntry `json:"packages"`
}

// Semver is a parsed semantic version.
type Semver struct {
	Major int
	Minor int
	Patch int
	Pre   string
}

// UpdateInfo describes an available update for an installed package.
type UpdateInfo struct {
	Name           string `json:"name"`
	CurrentVersion string `json:"current_version"`
	LatestVersion  string `json:"latest_version"`
	Repo           string `json:"repo"`
}

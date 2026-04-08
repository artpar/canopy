package registry

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"canopy/pkg/github"
)

// PublishResult is returned after a successful publish.
type PublishResult struct {
	Repo       string `json:"repo"`
	Tag        string `json:"tag"`
	ReleaseURL string `json:"release_url"`
}

// Publish creates a GitHub release for a package.
// The repo must already exist and contain the code. This function:
// 1. Validates canopy.json
// 2. Creates a git tag if it doesn't exist
// 3. Creates a GitHub release
// 4. Sets the canopy-package topic
func Publish(client *github.Client, localPath string, ownerRepo string, tagOverride string) (*PublishResult, error) {
	if !client.IsAuthenticated() {
		return nil, fmt.Errorf("not authenticated; run 'canopy pkg login' first")
	}

	// Read and validate canopy.json
	manifestPath := filepath.Join(localPath, "canopy.json")
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil, fmt.Errorf("read canopy.json: %w", err)
	}
	var manifest Manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("parse canopy.json: %w", err)
	}
	if err := validateManifest(&manifest); err != nil {
		return nil, err
	}

	// Determine tag
	tag := tagOverride
	if tag == "" {
		tag = "v" + manifest.Version
	}
	if !strings.HasPrefix(tag, "v") {
		tag = "v" + tag
	}

	// Infer repo from manifest if not provided
	if ownerRepo == "" {
		return nil, fmt.Errorf("repo is required for publish")
	}

	// Get HEAD SHA for tagging
	sha, err := client.GetDefaultBranchSHA(ownerRepo)
	if err != nil {
		return nil, fmt.Errorf("get branch SHA: %w", err)
	}

	// Create tag (ignore error if already exists)
	_ = client.CreateTag(ownerRepo, tag, sha)

	// Create release
	relBody := manifest.Description
	if relBody == "" {
		relBody = fmt.Sprintf("Release %s of %s", tag, manifest.Name)
	}
	release, err := client.CreateRelease(ownerRepo, github.CreateReleaseRequest{
		TagName: tag,
		Name:    fmt.Sprintf("%s %s", manifest.Name, tag),
		Body:    relBody,
	})
	if err != nil {
		return nil, fmt.Errorf("create release: %w", err)
	}

	// Set topics
	existingTopics, _ := client.GetTopics(ownerRepo)
	topics := ensureTopics(existingTopics, manifest.Type)
	_ = client.SetTopics(ownerRepo, topics)

	return &PublishResult{
		Repo:       ownerRepo,
		Tag:        tag,
		ReleaseURL: release.HTMLURL,
	}, nil
}

func validateManifest(m *Manifest) error {
	if m.Name == "" {
		return fmt.Errorf("canopy.json: name is required")
	}
	if m.Version == "" {
		return fmt.Errorf("canopy.json: version is required")
	}
	if _, err := ParseSemver(m.Version); err != nil {
		return fmt.Errorf("canopy.json: invalid version: %w", err)
	}
	if m.Type == "" {
		return fmt.Errorf("canopy.json: type is required")
	}
	switch m.Type {
	case TypeApp, TypeComponent, TypeTheme, TypeFFIConfig:
	default:
		return fmt.Errorf("canopy.json: unknown type %q", m.Type)
	}
	if m.Entry == "" {
		return fmt.Errorf("canopy.json: entry is required")
	}
	return nil
}

func ensureTopics(existing []string, pkgType PackageType) []string {
	topicSet := make(map[string]bool)
	for _, t := range existing {
		topicSet[t] = true
	}
	topicSet["canopy-package"] = true
	topicSet["canopy-"+string(pkgType)] = true

	result := make([]string, 0, len(topicSet))
	for t := range topicSet {
		result = append(result, t)
	}
	return result
}

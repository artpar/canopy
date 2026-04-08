package mcp

import (
	"encoding/json"
	"fmt"
	"strings"

	"canopy/pkg/github"
	"canopy/pkg/registry"
)

func (s *Server) registerPackageTools() {
	if s.registry == nil {
		return
	}
	s.registerPackageLogin()
	s.registerPackageSearch()
	s.registerPackageBrowse()
	s.registerPackageInfo()
	s.registerPackageInstall()
	s.registerPackageUninstall()
	s.registerPackageUpdate()
	s.registerPackageList()
	s.registerPackagePublish()
}

func (s *Server) registerPackageLogin() {
	s.register("package_login", "Authenticate with GitHub for package management", json.RawMessage(`{
		"type": "object",
		"properties": {},
		"additionalProperties": false
	}`), func(args json.RawMessage) *ToolCallResult {
		tok, err := github.DeviceFlowLogin(func(userCode, verificationURI string) {
			// MCP clients can't show interactive prompts, so return the info as text
			_ = userCode
			_ = verificationURI
		})
		if err != nil {
			return errorResult("login failed: " + err.Error())
		}

		return textResult(fmt.Sprintf("Authenticated with GitHub (scope: %s). Token saved.", tok.Scope))
	})
}

func (s *Server) registerPackageSearch() {
	s.register("package_search", "Search for Canopy packages on GitHub", json.RawMessage(`{
		"type": "object",
		"properties": {
			"query":  {"type": "string", "description": "Search query"},
			"type":   {"type": "string", "enum": ["app", "component", "theme", "ffi-config"], "description": "Filter by package type"},
			"limit":  {"type": "integer", "description": "Max results (default 20)"}
		},
		"required": ["query"],
		"additionalProperties": false
	}`), func(args json.RawMessage) *ToolCallResult {
		var p struct {
			Query string `json:"query"`
			Type  string `json:"type"`
			Limit int    `json:"limit"`
		}
		if err := json.Unmarshal(args, &p); err != nil {
			return errorResult("invalid args: " + err.Error())
		}

		client, err := github.NewClientFromStored()
		if err != nil {
			return errorResult("github client: " + err.Error())
		}

		result, err := client.SearchRepos(p.Query, p.Type, p.Limit)
		if err != nil {
			return errorResult("search: " + err.Error())
		}

		data, _ := json.Marshal(result)
		return &ToolCallResult{Content: []ContentBlock{{Type: "text", Text: string(data)}}}
	})
}

func (s *Server) registerPackageBrowse() {
	s.register("package_browse", "List popular Canopy packages", json.RawMessage(`{
		"type": "object",
		"properties": {
			"type":  {"type": "string", "enum": ["app", "component", "theme", "ffi-config"], "description": "Filter by package type"},
			"sort":  {"type": "string", "enum": ["stars", "updated"], "description": "Sort order (default: stars)"},
			"limit": {"type": "integer", "description": "Max results (default 20)"}
		},
		"additionalProperties": false
	}`), func(args json.RawMessage) *ToolCallResult {
		var p struct {
			Type  string `json:"type"`
			Sort  string `json:"sort"`
			Limit int    `json:"limit"`
		}
		if err := json.Unmarshal(args, &p); err != nil {
			return errorResult("invalid args: " + err.Error())
		}

		client, err := github.NewClientFromStored()
		if err != nil {
			return errorResult("github client: " + err.Error())
		}

		result, err := client.BrowseRepos(p.Type, p.Sort, p.Limit)
		if err != nil {
			return errorResult("browse: " + err.Error())
		}

		data, _ := json.Marshal(result)
		return &ToolCallResult{Content: []ContentBlock{{Type: "text", Text: string(data)}}}
	})
}

func (s *Server) registerPackageInfo() {
	s.register("package_info", "Get details about a Canopy package", json.RawMessage(`{
		"type": "object",
		"properties": {
			"repo": {"type": "string", "description": "GitHub owner/repo"}
		},
		"required": ["repo"],
		"additionalProperties": false
	}`), func(args json.RawMessage) *ToolCallResult {
		var p struct {
			Repo string `json:"repo"`
		}
		if err := json.Unmarshal(args, &p); err != nil {
			return errorResult("invalid args: " + err.Error())
		}

		client, err := github.NewClientFromStored()
		if err != nil {
			return errorResult("github client: " + err.Error())
		}

		repo, err := client.GetRepo(p.Repo)
		if err != nil {
			return errorResult("get repo: " + err.Error())
		}

		info := map[string]any{
			"full_name":   repo.FullName,
			"description": repo.Description,
			"stars":       repo.Stars,
			"url":         repo.HTMLURL,
			"topics":      repo.Topics,
		}

		// Try to read canopy.json
		manifestData, err := client.GetFileContent(p.Repo, "canopy.json", "")
		if err == nil {
			var m registry.Manifest
			if json.Unmarshal(manifestData, &m) == nil {
				info["manifest"] = m
			}
		}

		// Get latest tag
		tags, err := client.ListTags(p.Repo)
		if err == nil && len(tags) > 0 {
			tagNames := make([]string, len(tags))
			for i, t := range tags {
				tagNames[i] = t.Name
			}
			latest, _, _ := registry.FindBestTag(tagNames, "")
			if latest != "" {
				info["latest_tag"] = latest
			}
		}

		// Check if installed locally
		entry := s.registry.Get(p.Repo)
		if entry != nil {
			info["installed"] = entry
		}

		data, _ := json.Marshal(info)
		return &ToolCallResult{Content: []ContentBlock{{Type: "text", Text: string(data)}}}
	})
}

func (s *Server) registerPackageInstall() {
	s.register("package_install", "Install a Canopy package from GitHub", json.RawMessage(`{
		"type": "object",
		"properties": {
			"repo":    {"type": "string", "description": "GitHub owner/repo"},
			"version": {"type": "string", "description": "Version constraint or tag (default: latest)"}
		},
		"required": ["repo"],
		"additionalProperties": false
	}`), func(args json.RawMessage) *ToolCallResult {
		var p struct {
			Repo    string `json:"repo"`
			Version string `json:"version"`
		}
		if err := json.Unmarshal(args, &p); err != nil {
			return errorResult("invalid args: " + err.Error())
		}

		client, err := github.NewClientFromStored()
		if err != nil {
			return errorResult("github client: " + err.Error())
		}

		entry, err := registry.Install(s.registry, client, p.Repo, p.Version)
		if err != nil {
			return errorResult("install: " + err.Error())
		}

		data, _ := json.Marshal(entry)
		return &ToolCallResult{Content: []ContentBlock{{Type: "text", Text: string(data)}}}
	})
}

func (s *Server) registerPackageUninstall() {
	s.register("package_uninstall", "Uninstall a Canopy package", json.RawMessage(`{
		"type": "object",
		"properties": {
			"name": {"type": "string", "description": "Package key (owner/repo)"}
		},
		"required": ["name"],
		"additionalProperties": false
	}`), func(args json.RawMessage) *ToolCallResult {
		var p struct {
			Name string `json:"name"`
		}
		if err := json.Unmarshal(args, &p); err != nil {
			return errorResult("invalid args: " + err.Error())
		}

		if err := registry.Uninstall(s.registry, p.Name); err != nil {
			return errorResult("uninstall: " + err.Error())
		}

		return textResult(fmt.Sprintf("Uninstalled %s", p.Name))
	})
}

func (s *Server) registerPackageUpdate() {
	s.register("package_update", "Update installed Canopy packages", json.RawMessage(`{
		"type": "object",
		"properties": {
			"name": {"type": "string", "description": "Package key to update (omit for all)"}
		},
		"additionalProperties": false
	}`), func(args json.RawMessage) *ToolCallResult {
		var p struct {
			Name string `json:"name"`
		}
		if err := json.Unmarshal(args, &p); err != nil {
			return errorResult("invalid args: " + err.Error())
		}

		client, err := github.NewClientFromStored()
		if err != nil {
			return errorResult("github client: " + err.Error())
		}

		updated, err := registry.Update(s.registry, client, p.Name)
		if err != nil {
			return errorResult("update: " + err.Error())
		}

		if len(updated) == 0 {
			return textResult("All packages are up to date.")
		}

		var lines []string
		for _, u := range updated {
			lines = append(lines, fmt.Sprintf("%s: %s -> %s", u.Name, u.CurrentVersion, u.LatestVersion))
		}
		return textResult("Updated:\n" + strings.Join(lines, "\n"))
	})
}

func (s *Server) registerPackageList() {
	s.register("package_list", "List installed Canopy packages", json.RawMessage(`{
		"type": "object",
		"properties": {
			"type": {"type": "string", "enum": ["app", "component", "theme", "ffi-config"], "description": "Filter by package type"}
		},
		"additionalProperties": false
	}`), func(args json.RawMessage) *ToolCallResult {
		var p struct {
			Type string `json:"type"`
		}
		if err := json.Unmarshal(args, &p); err != nil {
			return errorResult("invalid args: " + err.Error())
		}

		entries := s.registry.List(registry.PackageType(p.Type))
		data, _ := json.Marshal(entries)
		return &ToolCallResult{Content: []ContentBlock{{Type: "text", Text: string(data)}}}
	})
}

func (s *Server) registerPackagePublish() {
	s.register("package_publish", "Publish a Canopy package to GitHub", json.RawMessage(`{
		"type": "object",
		"properties": {
			"path": {"type": "string", "description": "Local directory containing canopy.json"},
			"repo": {"type": "string", "description": "GitHub owner/repo to publish to"},
			"tag":  {"type": "string", "description": "Version tag (reads from canopy.json if omitted)"}
		},
		"required": ["path"],
		"additionalProperties": false
	}`), func(args json.RawMessage) *ToolCallResult {
		var p struct {
			Path string `json:"path"`
			Repo string `json:"repo"`
			Tag  string `json:"tag"`
		}
		if err := json.Unmarshal(args, &p); err != nil {
			return errorResult("invalid args: " + err.Error())
		}

		client, err := github.NewClientFromStored()
		if err != nil {
			return errorResult("github client: " + err.Error())
		}

		result, err := registry.Publish(client, p.Path, p.Repo, p.Tag)
		if err != nil {
			return errorResult("publish: " + err.Error())
		}

		data, _ := json.Marshal(result)
		return &ToolCallResult{Content: []ContentBlock{{Type: "text", Text: string(data)}}}
	})
}

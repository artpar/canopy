package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"canopy/pkg/github"
	"canopy/pkg/registry"
)

// RunPkg handles the "canopy pkg <subcommand>" CLI.
func RunPkg(args []string) {
	if len(args) == 0 {
		printPkgUsage()
		os.Exit(1)
	}

	subcmd := args[0]
	rest := args[1:]

	switch subcmd {
	case "login":
		cmdLogin()
	case "search":
		cmdSearch(rest)
	case "info":
		cmdInfo(rest)
	case "install":
		cmdInstall(rest)
	case "uninstall":
		cmdUninstall(rest)
	case "update":
		cmdUpdate(rest)
	case "list":
		cmdList(rest)
	case "publish":
		cmdPublish(rest)
	case "help", "--help", "-h":
		printPkgUsage()
	default:
		fmt.Fprintf(os.Stderr, "unknown pkg subcommand: %s\n", subcmd)
		printPkgUsage()
		os.Exit(1)
	}
}

func printPkgUsage() {
	fmt.Println(`Usage: canopy pkg <command> [args]

Commands:
  login                         Authenticate with GitHub
  search <query> [--type=TYPE]  Search for Canopy packages
  info <owner/repo>             Show package details
  install <owner/repo> [@ver]   Install a package
  uninstall <owner/name>        Uninstall a package
  update [<owner/name>]         Update packages
  list [--type=TYPE]            List installed packages
  publish [path] [--repo=O/R]   Publish to GitHub

Types: app, component, theme, ffi-config`)
}

func cmdLogin() {
	tok, err := github.DeviceFlowLogin(func(userCode, verificationURI string) {
		fmt.Println()
		fmt.Println("  GitHub Device Authorization")
		fmt.Println()
		fmt.Printf("  1. Open: %s\n", verificationURI)
		fmt.Printf("  2. Enter code: %s\n", userCode)
		fmt.Println()
		fmt.Println("  Waiting for authorization...")
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "login failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Authenticated as GitHub user (token scope: %s)\n", tok.Scope)
}

func cmdSearch(args []string) {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: canopy pkg search <query> [--type=TYPE]")
		os.Exit(1)
	}

	query, pkgType := parseSearchArgs(args)

	client, err := github.NewClientFromStored()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	result, err := client.SearchRepos(query, pkgType, 20)
	if err != nil {
		fmt.Fprintf(os.Stderr, "search failed: %v\n", err)
		os.Exit(1)
	}

	if result.TotalCount == 0 {
		fmt.Println("No packages found.")
		return
	}

	fmt.Printf("Found %d package(s):\n\n", result.TotalCount)
	for _, repo := range result.Items {
		stars := ""
		if repo.Stars > 0 {
			stars = fmt.Sprintf(" (%d stars)", repo.Stars)
		}
		fmt.Printf("  %s%s\n", repo.FullName, stars)
		if repo.Description != "" {
			fmt.Printf("    %s\n", repo.Description)
		}
	}
}

func cmdInfo(args []string) {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: canopy pkg info <owner/repo>")
		os.Exit(1)
	}

	ownerRepo := args[0]
	client, err := github.NewClientFromStored()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	// Fetch repo metadata
	repo, err := client.GetRepo(ownerRepo)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	// Try to fetch canopy.json
	manifestData, err := client.GetFileContent(ownerRepo, "canopy.json", "")
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: no canopy.json found\n")
	}

	fmt.Printf("Repository: %s\n", repo.FullName)
	fmt.Printf("Description: %s\n", repo.Description)
	fmt.Printf("Stars: %d\n", repo.Stars)
	fmt.Printf("URL: %s\n", repo.HTMLURL)

	if manifestData != nil {
		var m registry.Manifest
		if json.Unmarshal(manifestData, &m) == nil {
			fmt.Printf("\nPackage:\n")
			fmt.Printf("  Name: %s\n", m.Name)
			fmt.Printf("  Version: %s\n", m.Version)
			fmt.Printf("  Type: %s\n", m.Type)
			fmt.Printf("  Entry: %s\n", m.Entry)
			if len(m.Keywords) > 0 {
				fmt.Printf("  Keywords: %s\n", strings.Join(m.Keywords, ", "))
			}
			if len(m.Dependencies) > 0 {
				fmt.Printf("  Dependencies:\n")
				for dep, constraint := range m.Dependencies {
					fmt.Printf("    %s %s\n", dep, constraint)
				}
			}
		}
	}

	// Show latest tag
	tags, err := client.ListTags(ownerRepo)
	if err == nil && len(tags) > 0 {
		tagNames := make([]string, len(tags))
		for i, t := range tags {
			tagNames[i] = t.Name
		}
		latest, _, _ := registry.FindBestTag(tagNames, "")
		if latest != "" {
			fmt.Printf("  Latest tag: %s\n", latest)
		}
	}
}

func cmdInstall(args []string) {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: canopy pkg install <owner/repo> [@version]")
		os.Exit(1)
	}

	ownerRepo := args[0]
	version := ""
	if len(args) > 1 {
		version = strings.TrimPrefix(args[1], "@")
	}

	client, err := github.NewClientFromStored()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	reg, err := registry.New()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Installing %s...\n", ownerRepo)
	entry, err := registry.Install(reg, client, ownerRepo, version)
	if err != nil {
		fmt.Fprintf(os.Stderr, "install failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Installed %s v%s (%s) to %s\n", entry.Name, entry.Version, entry.Type, entry.Path)
}

func cmdUninstall(args []string) {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: canopy pkg uninstall <owner/name>")
		os.Exit(1)
	}

	reg, err := registry.New()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	key := args[0]
	if err := registry.Uninstall(reg, key); err != nil {
		fmt.Fprintf(os.Stderr, "uninstall failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Uninstalled %s\n", key)
}

func cmdUpdate(args []string) {
	name := ""
	if len(args) > 0 {
		name = args[0]
	}

	client, err := github.NewClientFromStored()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	reg, err := registry.New()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	if name == "" {
		fmt.Println("Checking for updates...")
	} else {
		fmt.Printf("Checking for updates to %s...\n", name)
	}

	updated, err := registry.Update(reg, client, name)
	if err != nil {
		fmt.Fprintf(os.Stderr, "update failed: %v\n", err)
		os.Exit(1)
	}

	if len(updated) == 0 {
		fmt.Println("All packages are up to date.")
		return
	}

	for _, u := range updated {
		fmt.Printf("Updated %s: %s -> %s\n", u.Name, u.CurrentVersion, u.LatestVersion)
	}
}

func cmdList(args []string) {
	pkgType := ""
	for _, arg := range args {
		if strings.HasPrefix(arg, "--type=") {
			pkgType = strings.TrimPrefix(arg, "--type=")
		}
	}

	reg, err := registry.New()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	entries := reg.List(registry.PackageType(pkgType))
	if len(entries) == 0 {
		fmt.Println("No packages installed.")
		return
	}

	fmt.Printf("Installed packages (%d):\n\n", len(entries))
	for _, e := range entries {
		fmt.Printf("  %s v%s (%s)\n", e.Repo, e.Version, e.Type)
		fmt.Printf("    Path: %s\n", e.Path)
	}
}

func cmdPublish(args []string) {
	path := "."
	ownerRepo := ""
	tag := ""

	for _, arg := range args {
		if strings.HasPrefix(arg, "--repo=") {
			ownerRepo = strings.TrimPrefix(arg, "--repo=")
		} else if strings.HasPrefix(arg, "--tag=") {
			tag = strings.TrimPrefix(arg, "--tag=")
		} else if !strings.HasPrefix(arg, "-") {
			path = arg
		}
	}

	client, err := github.NewClientFromStored()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	if !client.IsAuthenticated() {
		fmt.Fprintln(os.Stderr, "Not authenticated. Run 'canopy pkg login' first.")
		os.Exit(1)
	}

	if ownerRepo == "" {
		fmt.Fprintln(os.Stderr, "usage: canopy pkg publish [path] --repo=owner/repo")
		os.Exit(1)
	}

	fmt.Printf("Publishing %s to %s...\n", path, ownerRepo)
	result, err := registry.Publish(client, path, ownerRepo, tag)
	if err != nil {
		fmt.Fprintf(os.Stderr, "publish failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Published %s %s\n", result.Repo, result.Tag)
	fmt.Printf("Release: %s\n", result.ReleaseURL)
}

func parseSearchArgs(args []string) (query string, pkgType string) {
	var queryParts []string
	for _, arg := range args {
		if strings.HasPrefix(arg, "--type=") {
			pkgType = strings.TrimPrefix(arg, "--type=")
		} else {
			queryParts = append(queryParts, arg)
		}
	}
	return strings.Join(queryParts, " "), pkgType
}

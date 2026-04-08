---
layout: default
title: Bundling & Distribution
parent: Building Apps
nav_order: 13
---

# Bundling & Distribution

`canopy bundle` creates standalone macOS `.app` bundles from any Canopy app. The result is a self-contained application --- double-click to launch, no CLI needed.

## Basic Bundling

```bash
canopy bundle myapp/
```

This creates `MyApp.app` in the current directory with this structure:

```
MyApp.app/
  Contents/
    Info.plist
    MacOS/
      canopy          # copy of the canopy binary
    Resources/
      app/            # your app files (JSONL, assets)
      AppIcon.icns    # app icon (if provided)
```

### Flags

| Flag | Description |
|:-----|:------------|
| `--output`, `-o` | Output path (default: `./<AppName>.app`) |
| `--name` | Override app name (default: from `canopy.json` or directory name) |
| `--icon` | Path to `.icns` file for the app icon |
| `--bundle-id` | macOS bundle identifier (default: `com.canopy.app.<name>`) |

```bash
canopy bundle -o ~/Desktop/Notes.app sample_apps/notes
canopy bundle --name "My Notes" --icon icon.icns myapp/
canopy bundle --bundle-id com.example.myapp myapp/
```

### Using canopy.json

If your app directory contains a `canopy.json` manifest, the bundle command reads metadata from it automatically:

```json
{
  "name": "My Notes",
  "version": "2.1.0",
  "icon": "icon.icns",
  "bundleId": "com.example.notes",
  "entry": "app.jsonl"
}
```

Flag overrides take precedence over manifest values.

## Signing

macOS Gatekeeper controls what apps can run. There are three levels of signing, each requiring different setup:

| Level | Flag | Apple Account? | Other Machines |
|:------|:-----|:---------------|:---------------|
| Unsigned | *(none)* | No | Blocked by Gatekeeper |
| Ad-hoc | `--sign --identity "-"` | No | Right-click > Open first time |
| Developer ID | `--sign` | Yes ($99/yr) | Gatekeeper warning, then allowed |
| Developer ID + Notarized | `--sign --notarize` | Yes | No warnings at all |

### Ad-hoc Signing (No Account Needed)

```bash
canopy bundle --sign --identity "-" myapp/
```

This signs the app with hardened runtime and entitlements but without a certificate. The app runs fine on your machine. On other machines, the user must right-click the app and choose "Open" the first time.

### Developer ID Signing

```bash
canopy bundle --sign myapp/
```

Without `--identity`, the command auto-detects a `Developer ID Application` certificate from your keychain. This requires an [Apple Developer Program](https://developer.apple.com/programs/) membership ($99/year).

To use a specific identity:

```bash
canopy bundle --sign --identity "Developer ID Application: Your Name (TEAMID)" myapp/
```

## Notarization

Notarization submits your signed app to Apple for automated security scanning. Once approved, Apple issues a ticket that macOS checks --- the app opens with zero Gatekeeper warnings on any machine.

```bash
canopy bundle --sign --notarize myapp/
```

The `--notarize` flag implies `--sign`. The process takes 2--15 minutes as Apple scans the binary.

### Providing Credentials

Notarization requires Apple credentials. Three ways to provide them:

**1. Keychain profile (recommended):**

First, store credentials once:

```bash
xcrun notarytool store-credentials myprofile \
  --apple-id you@example.com \
  --team-id ABC123 \
  --password <app-specific-password>
```

Then use the profile:

```bash
canopy bundle --sign --notarize --keychain-profile myprofile myapp/
```

**2. Environment variables:**

```bash
export CANOPY_APPLE_ID=you@example.com
export CANOPY_TEAM_ID=ABC123
export CANOPY_APP_PASSWORD=xxxx-xxxx-xxxx-xxxx

canopy bundle --sign --notarize myapp/
```

**3. Flags:**

```bash
canopy bundle --sign --notarize \
  --apple-id you@example.com \
  --team-id ABC123 \
  --password xxxx-xxxx-xxxx-xxxx \
  myapp/
```

{: .tip }
> Create an app-specific password at [appleid.apple.com](https://appleid.apple.com/account/manage) under Sign-In and Security > App-Specific Passwords.

## Entitlements

The bundle command embeds a hardened runtime entitlements file with these capabilities:

| Entitlement | Why |
|:------------|:----|
| `com.apple.security.device.camera` | CameraView component, headless photo capture |
| `com.apple.security.device.audio-input` | AudioRecorder component, headless audio recording |
| `com.apple.security.network.client` | LLM provider HTTP calls, httpGet/httpPost functions |
| `com.apple.security.files.user-selected.read-write` | fileOpen/fileSave dialogs (NSOpenPanel/NSSavePanel) |
| `com.apple.security.cs.allow-unsigned-executable-memory` | Required by CGo runtime |
| `com.apple.security.cs.disable-library-validation` | FFI dylib loading via loadLibrary |

These are always included because the canopy binary supports all these features. The entitlements file is embedded in the binary itself --- no external files needed.

## Bundling Installed Packages

If you have installed a package via `canopy pkg install`, you can bundle it directly by `owner/repo`:

```bash
canopy pkg install artpar/calculator
canopy bundle artpar/calculator
```

This resolves through the local registry to `~/.canopy/apps/artpar/calculator/` and bundles from there.

## How Bundled Apps Work

When a bundled `.app` launches, the canopy binary detects that it is running inside a `.app/Contents/MacOS/` path. It then:

1. Looks for `../Resources/app/` relative to the binary
2. Loads that directory as the app source (same as `canopy myapp/`)
3. Runs as a **normal dock application** (not the menubar tray)
4. Shows the app name from `canopy.json` in the splash window

The bundled app is the full canopy binary --- it still supports MCP tools, background processes, channels, and all system capabilities. The only difference is the startup mode.

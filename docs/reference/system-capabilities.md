---
layout: default
title: System Capabilities
parent: Reference
nav_order: 1
---

# System Capabilities

Canopy exposes 16 native macOS functions. Each function is available in two ways:

1. **Evaluator functions** -- callable from JSONL expressions inside your app (e.g., in `onClick` actions, `functionCall` messages)
2. **MCP tools** -- callable programmatically via the MCP server (see [MCP Tools](../mcp/))

## Functions

| Function | Args | Returns | Description |
|----------|------|---------|-------------|
| `notify` | title, body, subtitle? | `"sent"` | Show a macOS notification via Notification Center |
| `clipboardRead` | -- | text | Read the current system clipboard contents |
| `clipboardWrite` | text | `"copied"` | Write text to the system clipboard |
| `openURL` | url | `"opened"` | Open a URL or file path in the default application |
| `fileOpen` | title?, types?, multi? | path(s) or `""` | Show a native file open dialog (NSOpenPanel) |
| `fileSave` | title?, name?, types? | path or `""` | Show a native file save dialog (NSSavePanel) |
| `alert` | title, msg, style?, buttons? | button index | Show a native alert dialog (NSAlert) |
| `httpGet` | url | response body | Make an HTTP GET request (30s timeout) |
| `httpPost` | url, body, type? | response body | Make an HTTP POST request (30s timeout) |
| `cameraCapture` | devicePosition? | file path | Take a photo, returns path to JPEG |
| `audioRecordStart` | format?, sampleRate?, channels? | recording ID | Start microphone recording |
| `audioRecordStop` | recordingID | file path | Stop recording, returns path to audio file |
| `screenCapture` | captureType? | file path | Capture the screen, returns path to PNG |
| `screenRecordStart` | captureType? | recording ID | Start screen recording |
| `screenRecordStop` | recordingID | file path | Stop screen recording, returns path to video |

## Using in JSONL

Functions are called from action handlers using `functionCall`:

```json
{
  "type": "functionCall",
  "name": "notify",
  "args": [
    "Download complete",
    "Your file has been saved."
  ]
}
```

Or as expressions in component props:

```json
{
  "componentId": "copy-btn",
  "type": "Button",
  "props": {
    "label": "Copy",
    "onClick": {
      "action": {
        "functionCall": {
          "name": "clipboardWrite",
          "args": [
            "${/data/selectedText}"
          ]
        }
      }
    }
  }
}
```

## Using as MCP Tools

The same functions are available as MCP tools with slightly different naming (snake_case):

| Evaluator function | MCP tool |
|-------------------|----------|
| `notify` | `notify` |
| `clipboardRead` | `clipboard_read` |
| `clipboardWrite` | `clipboard_write` |
| `openURL` | `open_url` |
| `fileOpen` | `file_open` |
| `fileSave` | `file_save` |
| `alert` | `alert` |
| `cameraCapture` | `camera_capture_headless` |
| `audioRecordStart` | `audio_record_start` |
| `audioRecordStop` | `audio_record_stop` |
| `screenCapture` | `screen_capture` |

See [System Capabilities MCP Tools](../mcp/system-tools) and [Media Capture MCP Tools](../mcp/media-tools) for full parameter details.

## Threading

File dialogs (`fileOpen`, `fileSave`) and alerts use `beginWithCompletionHandler:` / `beginSheetModalForWindow:`. The main thread is never blocked -- the AppKit run loop continues processing rendering, MCP tools, and callbacks while a dialog is open. The calling goroutine blocks on a Go channel until the user dismisses the dialog.

## Permissions

Some functions require macOS permissions:

| Function | Permission | Info.plist key |
|----------|-----------|----------------|
| `cameraCapture` | Camera access | `NSCameraUsageDescription` |
| `audioRecordStart` | Microphone access | `NSMicrophoneUsageDescription` |
| `screenCapture` | Screen recording | Prompted by ScreenCaptureKit |

The system prompts the user for permission on first use. These keys are included in Canopy's `Info.plist`.

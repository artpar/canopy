---
layout: default
title: Media Capture
parent: MCP Tools
nav_order: 7
---

# Media Capture

These tools control the camera, microphone, and screen capture. Some work with visible components (CameraView, AudioRecorder), while others work headlessly without any UI.

---

## camera_capture

Take a photo from a CameraView component that is currently displayed in the app.

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `surface_id` | string | yes | The window containing the CameraView |
| `component_id` | string | yes | The CameraView component ID |

**Returns:** File path to the captured JPEG image.

**Example:**
```
mcp__canopy__camera_capture(surface_id: "main", component_id: "camera")
```

---

## camera_capture_headless

Take a photo without any visible UI. Uses the system camera directly.

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `devicePosition` | string | no | `"front"` or `"back"` (default: `"front"`) |

**Returns:** File path to the captured JPEG image.

**Example:**
```
mcp__canopy__camera_capture_headless(devicePosition: "front")
```

The photo is saved to a temporary file and the path is returned.

---

## audio_record_start

Start recording audio from the microphone.

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `format` | string | no | Audio format (default: system default) |
| `sampleRate` | number | no | Sample rate in Hz |
| `channels` | number | no | Number of channels (1 for mono, 2 for stereo) |

**Returns:** A recording ID to use with `audio_record_stop`.

**Example:**
```
mcp__canopy__audio_record_start(sampleRate: 44100, channels: 1)
```

```json
{
  "recordingID": "rec_abc123"
}
```

---

## audio_record_stop

Stop an active audio recording and get the saved file.

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `recordingID` | string | yes | The recording ID from `audio_record_start` |

**Returns:** File path to the recorded audio file.

**Example:**
```
mcp__canopy__audio_record_stop(recordingID: "rec_abc123")
```

```json
{
  "path": "/tmp/canopy_audio_rec_abc123.m4a"
}
```

---

## audio_recorder_toggle

Toggle recording on an AudioRecorder component in the UI.

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `surface_id` | string | yes | The window containing the AudioRecorder |
| `component_id` | string | yes | The AudioRecorder component ID |

**Example:**
```
mcp__canopy__audio_recorder_toggle(surface_id: "main", component_id: "recorder")
```

If the recorder is idle, it starts recording. If it is recording, it stops and saves the file.

---

## screen_capture

Capture a screenshot of the screen (not just the Canopy window).

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `captureType` | string | no | Capture mode (default: full screen) |

**Returns:** File path to the captured PNG image.

**Example:**
```
mcp__canopy__screen_capture()
```

```json
{
  "path": "/tmp/canopy_screen_capture.png"
}
```

Uses ScreenCaptureKit for high-quality capture. A system permission prompt may appear on first use.

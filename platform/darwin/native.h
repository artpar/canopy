#pragma once

// Notifications
void JVSendNotification(const char *title, const char *body, const char *subtitle);

// Clipboard
const char* JVClipboardRead(void);
void JVClipboardWrite(const char *text);

// Open URL/file in default app
void JVOpenURL(const char *url);

// File dialogs — return JSON-encoded results. Caller must free().
// Returns NULL if user cancelled.
const char* JVFileOpenPanel(const char *title, const char *allowedTypes, int allowMultiple);
const char* JVFileSavePanel(const char *title, const char *defaultName, const char *allowedTypes);

// Alert — returns clicked button index (0-based).
int JVAlert(const char *title, const char *message, const char *style, const char **buttons, int buttonCount);

#pragma once
#include <stdint.h>

// Notifications
void JVSendNotification(const char *title, const char *body, const char *subtitle);

// Clipboard
const char* JVClipboardRead(void);
void JVClipboardWrite(const char *text);

// Open URL/file in default app
void JVOpenURL(const char *url);

// File dialogs — async, non-blocking.
// Result delivered via GoNativeDialogResult(requestID, result).
// result is NULL if cancelled, JSON array of paths for open, path string for save.
void JVFileOpenPanelAsync(const char *title, const char *allowedTypes, int allowMultiple, uint64_t requestID);
void JVFileSavePanelAsync(const char *title, const char *defaultName, const char *allowedTypes, uint64_t requestID);

// Alert — async, non-blocking.
// Result delivered via GoNativeDialogResult(requestID, buttonIndexStr).
void JVAlertAsync(const char *title, const char *message, const char *style,
                  const char **buttons, int buttonCount, uint64_t requestID);

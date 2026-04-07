#ifndef JVIEW_APP_H
#define JVIEW_APP_H

#include <stdint.h>

void JVAppInit(void);
void JVAppRun(void);
void JVAppStop(void);
void JVAppRunUntilIdle(void);
void JVForceLayout(const char* surfaceID);
void* JVCreateWindow(const char* title, int width, int height, const char* surfaceID, const char* backgroundColor);
void JVDestroyWindow(const char* surfaceID);
void JVSetWindowRootView(const char* surfaceID, void* view, int padding);
void JVSetWindowTheme(const char* surfaceID, const char* theme);
void JVRemoveView(void* view);
void JVUpdateWindow(const char* surfaceID, const char* title, int minWidth, int minHeight);

// App mode: "normal", "menubar", "accessory"
void JVSetAppMode(const char* mode, const char* icon, const char* title, uint64_t callbackID);

// Set dynamic items in the status bar menu (JSON array of menu item specs)
void JVSetStatusMenuDynamic(const char* itemsJSON);

// Set the "Apps" submenu in the status bar menu (JSON array of {label, path, icon})
void JVSetStatusMenuApps(const char* itemsJSON);

void JVShowSplashWindow(const char* title, int width, int height);
void JVUpdateSplashStatus(const char* status);
void JVDismissSplash(void);

// Follow-up prompt panel (Cmd+L)
void JVShowFollowUpPanel(uint64_t requestID);
void JVSetFollowUpEnabled(int enabled);

#endif

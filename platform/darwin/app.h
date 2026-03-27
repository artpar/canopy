#ifndef JVIEW_APP_H
#define JVIEW_APP_H

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

void JVShowSplashWindow(const char* title, int width, int height);
void JVUpdateSplashStatus(const char* status);
void JVDismissSplash(void);

#endif

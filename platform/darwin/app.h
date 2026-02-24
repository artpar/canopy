#ifndef JVIEW_APP_H
#define JVIEW_APP_H

void JVAppInit(void);
void JVAppRun(void);
void JVAppStop(void);
void JVAppRunUntilIdle(void);
void JVForceLayout(const char* surfaceID);
void* JVCreateWindow(const char* title, int width, int height, const char* surfaceID);
void JVDestroyWindow(const char* surfaceID);
void JVSetWindowRootView(const char* surfaceID, void* view);

#endif

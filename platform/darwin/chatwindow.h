#ifndef JVIEW_CHATWINDOW_H
#define JVIEW_CHATWINDOW_H

#include <stdbool.h>

void JVShowChatWindow(void);
void JVHideChatWindow(void);
void JVChatAddUserMessage(const char* text);
void JVChatAddStatusMessage(const char* text);
void JVChatSetBusy(bool busy);

#endif

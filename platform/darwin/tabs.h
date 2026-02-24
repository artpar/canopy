#ifndef JVIEW_TABS_H
#define JVIEW_TABS_H

#include <stdint.h>

void* JVCreateTabs(const char** labels, int count, const char* activeTab, uint64_t callbackID);
void JVUpdateTabs(void* handle, const char** labels, int count, const char* activeTab);
void JVTabsSetChildren(void* handle, void** children, int count);
void JVTabsSetChildIDs(void* handle, const char** childIDs, int count);

#endif

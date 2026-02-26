#ifndef JVIEW_OUTLINEVIEW_H
#define JVIEW_OUTLINEVIEW_H

#include <stdint.h>

void* JVCreateOutlineView(const char* dataJSON, const char* labelKey,
                           const char* childrenKey, const char* iconKey,
                           const char* idKey, const char* selectedID,
                           const char* badgeKey, uint64_t callbackID);
void JVUpdateOutlineView(void* handle, const char* dataJSON,
                          const char* selectedID);

#endif

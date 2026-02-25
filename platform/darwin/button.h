#ifndef JVIEW_BUTTON_H
#define JVIEW_BUTTON_H

#include <stdint.h>
#include <stdbool.h>

void* JVCreateButton(const char* label, const char* style, bool disabled, uint64_t callbackID);
void JVUpdateButton(void* handle, const char* label, const char* style, bool disabled);
void JVUpdateButtonCallbackID(void* handle, uint64_t callbackID);

#endif

#ifndef JVIEW_CLICKGESTURE_H
#define JVIEW_CLICKGESTURE_H

#include <stdint.h>

void JVAttachClickGesture(void* handle, uint64_t callbackID);
void JVUpdateClickGestureCallbackID(void* handle, uint64_t callbackID);

#endif

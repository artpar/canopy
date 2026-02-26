#ifndef JVIEW_RICHTEXTEDITOR_H
#define JVIEW_RICHTEXTEDITOR_H

#include <stdint.h>
#include <stdbool.h>

void* JVCreateRichTextEditor(const char* content, bool editable, uint64_t callbackID);
void JVUpdateRichTextEditor(void* handle, const char* content, bool editable);
void JVRichTextEditorSetFormatCallbackID(void* handle, uint64_t callbackID);

#endif

#ifndef JVIEW_TEXT_H
#define JVIEW_TEXT_H

void* JVCreateText(const char* content, const char* variant, int maxLines);
void JVUpdateText(void* handle, const char* content, const char* variant, int maxLines);

#endif

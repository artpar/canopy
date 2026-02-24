#ifndef JVIEW_ASSETS_H
#define JVIEW_ASSETS_H

// JVRegisterFont registers a font file (local path or URL) with the system for process scope.
void JVRegisterFont(const char* src);

// JVPreloadImage downloads and caches an image by alias.
void JVPreloadImage(const char* alias, const char* src);

// JVGetCachedImage returns a cached NSImage for the given alias, or NULL.
void* JVGetCachedImage(const char* alias);

#endif

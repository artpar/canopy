#ifndef JVIEW_CAMERA_H
#define JVIEW_CAMERA_H

#include <stdint.h>
#include <stdbool.h>

void* JVCreateCamera(const char* devicePosition, bool mirrored, uint64_t captureCbID, uint64_t errorCbID);
void JVUpdateCamera(void* handle, const char* devicePosition, bool mirrored);
void JVCameraCapture(void* handle);
void JVCleanupCamera(void* handle);

#endif

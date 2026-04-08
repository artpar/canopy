#ifndef JVIEW_AUDIORECORDER_H
#define JVIEW_AUDIORECORDER_H

#include <stdint.h>
#include <stdbool.h>

void* JVCreateAudioRecorder(const char* format, double sampleRate, int channels,
    uint64_t startedCbID, uint64_t stoppedCbID, uint64_t levelCbID, uint64_t errorCbID);
void JVUpdateAudioRecorder(void* handle, const char* format, double sampleRate, int channels);
void JVAudioRecorderToggle(void* handle);
void JVCleanupAudioRecorder(void* handle);

#endif

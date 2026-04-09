#ifndef JVIEW_SENSORS_H
#define JVIEW_SENSORS_H

// Tier 1: No permissions needed, high value
void JVStartBatterySensor(int intervalMs);
void JVStopBatterySensor(void);

void JVStartMemorySensor(int intervalMs);
void JVStopMemorySensor(void);

void JVStartCPUSensor(int intervalMs);
void JVStopCPUSensor(void);

void JVStartDiskSensor(int intervalMs);
void JVStopDiskSensor(void);

// Tier 2: Medium complexity
void JVStartNetworkThroughputSensor(int intervalMs);
void JVStopNetworkThroughputSensor(void);

void JVStartAudioSensor(int intervalMs);
void JVStopAudioSensor(void);

void JVStartDisplaySensor(int intervalMs);
void JVStopDisplaySensor(void);

void JVStartActiveAppSensor(int intervalMs);
void JVStopActiveAppSensor(void);

void JVStartUptimeSensor(int intervalMs);
void JVStopUptimeSensor(void);

#endif

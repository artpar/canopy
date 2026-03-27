#ifndef JVIEW_PROGRESSBAR_H
#define JVIEW_PROGRESSBAR_H

#include <stdbool.h>

void* JVCreateProgressBar(double min, double max, double value, bool indeterminate);
void JVUpdateProgressBar(void* handle, double min, double max, double value, bool indeterminate);

#endif

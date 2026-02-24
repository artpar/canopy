#ifndef JVIEW_STYLE_H
#define JVIEW_STYLE_H

void JVApplyStyle(void* handle, const char* bg, const char* tc,
    double cornerRadius, double width, double height,
    double fontSize, const char* fontWeight, const char* textAlign, double opacity,
    const char* fontFamily);

#endif

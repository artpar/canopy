#ifndef JVIEW_VIEWQUERY_H
#define JVIEW_VIEWQUERY_H

// JVViewFrame stores the frame rectangle of a view.
typedef struct {
    double x;
    double y;
    double width;
    double height;
} JVViewFrame;

// JVViewStyle stores computed style properties of a view.
typedef struct {
    const char* fontName;
    double fontSize;
    int bold;
    int italic;
    const char* textColor;  // hex "#RRGGBB" or empty
    const char* bgColor;    // hex "#RRGGBB" or empty
    int hidden;
    double opacity;
} JVViewStyle;

// Query the frame of a view in window coordinates.
JVViewFrame JVGetViewFrame(void* handle);

// Query the computed style of a view.
JVViewStyle JVGetViewStyle(void* handle);

// Free strings allocated by JVGetViewStyle.
void JVFreeViewStyle(JVViewStyle style);

#endif
